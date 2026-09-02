package app

import (
	"newsfall/internal/config"
	"newsfall/internal/content"
	"newsfall/internal/model"
	"newsfall/internal/ui"
)

// Controller owns navigation and article assignment without terminal or I/O
// concerns, making runtime behavior deterministic and testable.
type Controller struct {
	Config         config.Config
	Articles       []model.Article
	Columns        []ui.ColumnView
	Active         int
	StreamSelected int
	NewIDs         map[string]bool
}

func NewController(cfg config.Config, articles []model.Article) *Controller {
	if err := config.Validate(&cfg); err != nil {
		cfg = config.Default()
	}
	controller := &Controller{Config: cfg, Articles: content.Deduplicate(articles), NewIDs: make(map[string]bool)}
	controller.trim()
	controller.rebuild("", nil, "")
	return controller
}

func (c *Controller) SetConfig(cfg config.Config) {
	activeID := c.activeColumnID()
	selected := c.selectedIDs()
	streamID := c.streamSelectedID()
	c.Config = cfg
	c.trim()
	c.rebuild(activeID, selected, streamID)
}

func (c *Controller) MoveHorizontal(delta int) {
	if len(c.Columns) == 0 {
		c.Active = 0
		return
	}
	c.Active = normalize(c.Active+delta, len(c.Columns))
}

func (c *Controller) MoveVertical(delta int) {
	if c.Config.Mode == "stream" {
		if len(c.Articles) > 0 {
			c.StreamSelected = normalize(c.StreamSelected+delta, len(c.Articles))
		}
		return
	}
	if len(c.Columns) == 0 {
		return
	}
	view := &c.Columns[normalize(c.Active, len(c.Columns))]
	if len(view.Articles) > 0 {
		view.Selected = normalize(view.Selected+delta, len(view.Articles))
	}
}

func (c *Controller) First() {
	if c.Config.Mode == "stream" {
		c.StreamSelected = 0
		return
	}
	if len(c.Columns) > 0 {
		c.Columns[normalize(c.Active, len(c.Columns))].Selected = 0
	}
}

func (c *Controller) Last() {
	if c.Config.Mode == "stream" {
		if len(c.Articles) > 0 {
			c.StreamSelected = len(c.Articles) - 1
		}
		return
	}
	if len(c.Columns) > 0 {
		view := &c.Columns[normalize(c.Active, len(c.Columns))]
		if len(view.Articles) > 0 {
			view.Selected = len(view.Articles) - 1
		}
	}
}

func (c *Controller) SelectedArticle() (model.Article, bool) {
	if c.Config.Mode == "stream" {
		if len(c.Articles) == 0 {
			return model.Article{}, false
		}
		return c.Articles[normalize(c.StreamSelected, len(c.Articles))], true
	}
	if len(c.Columns) == 0 {
		return model.Article{}, false
	}
	view := c.Columns[normalize(c.Active, len(c.Columns))]
	if len(view.Articles) == 0 {
		return model.Article{}, false
	}
	return view.Articles[normalize(view.Selected, len(view.Articles))], true
}

// Merge combines a refresh with the cache, marks genuinely unseen fetched
// stories, and keeps the currently selected article stable where possible.
func (c *Controller) Merge(fetched []model.Article) int {
	known := make(map[string]struct{}, len(c.Articles))
	for _, article := range c.Articles {
		known[article.ID] = struct{}{}
	}
	c.NewIDs = make(map[string]bool)
	for _, article := range fetched {
		if _, exists := known[article.ID]; !exists {
			c.NewIDs[article.ID] = true
		}
	}
	activeID := c.activeColumnID()
	selected := c.selectedIDs()
	streamID := c.streamSelectedID()
	combined := make([]model.Article, 0, len(fetched)+len(c.Articles))
	combined = append(combined, fetched...)
	combined = append(combined, c.Articles...)
	c.Articles = content.Deduplicate(combined)
	c.trim()
	c.rebuild(activeID, selected, streamID)
	return len(c.NewIDs)
}

func (c *Controller) Drift() {
	for i := range c.Columns {
		if len(c.Columns[i].Articles) > 0 {
			c.Columns[i].Selected = normalize(c.Columns[i].Selected+1, len(c.Columns[i].Articles))
		}
	}
	if len(c.Articles) > 0 {
		c.StreamSelected = normalize(c.StreamSelected+1, len(c.Articles))
	}
}

func (c *Controller) CycleTheme() {
	themes := []string{"aurora", "ember", "ocean", "mono"}
	index := 0
	for i, theme := range themes {
		if c.Config.Theme == theme {
			index = i
			break
		}
	}
	c.Config.Theme = themes[(index+1)%len(themes)]
}

func (c *Controller) trim() {
	limit := c.Config.MaxItems
	if limit > 0 && len(c.Articles) > limit {
		c.Articles = c.Articles[:limit]
	}
}

func (c *Controller) rebuild(activeID string, selected map[string]string, streamID string) {
	assigned := content.Assign(c.Articles, c.Config.Columns, c.Config.MaxPerColumn)
	views := make([]ui.ColumnView, 0, len(c.Config.Columns))
	for _, column := range c.Config.Columns {
		view := ui.ColumnView{Column: column, Articles: assigned[column.ID]}
		if selectedID := selected[column.ID]; selectedID != "" {
			view.Selected = findArticle(view.Articles, selectedID)
		}
		views = append(views, view)
	}
	c.Columns = views
	c.Active = findColumn(views, activeID)
	if streamID != "" {
		c.StreamSelected = findArticle(c.Articles, streamID)
	}
	if len(c.Articles) == 0 {
		c.StreamSelected = 0
	} else {
		c.StreamSelected = normalize(c.StreamSelected, len(c.Articles))
	}
}

func (c *Controller) selectedIDs() map[string]string {
	out := make(map[string]string, len(c.Columns))
	for _, view := range c.Columns {
		if len(view.Articles) > 0 {
			out[view.Column.ID] = view.Articles[normalize(view.Selected, len(view.Articles))].ID
		}
	}
	return out
}

func (c *Controller) activeColumnID() string {
	if len(c.Columns) == 0 {
		return ""
	}
	return c.Columns[normalize(c.Active, len(c.Columns))].Column.ID
}

func (c *Controller) streamSelectedID() string {
	if len(c.Articles) == 0 {
		return ""
	}
	return c.Articles[normalize(c.StreamSelected, len(c.Articles))].ID
}

func findArticle(articles []model.Article, id string) int {
	for i, article := range articles {
		if article.ID == id {
			return i
		}
	}
	return 0
}

func findColumn(columns []ui.ColumnView, id string) int {
	for i, column := range columns {
		if column.Column.ID == id {
			return i
		}
	}
	return 0
}

func normalize(value, length int) int {
	if length <= 0 {
		return 0
	}
	value %= length
	if value < 0 {
		value += length
	}
	return value
}
