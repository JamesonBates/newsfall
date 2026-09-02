package ui

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"newsfall/internal/command"
	"newsfall/internal/config"
	"newsfall/internal/content"
	"newsfall/internal/model"
)

// ColumnView contains the independently scrollable state for one deck column.
type ColumnView struct {
	Column   config.Column
	Articles []model.Article
	Selected int
}

// State is a complete immutable rendering snapshot.
type State struct {
	Width          int
	Height         int
	Now            time.Time
	Config         config.Config
	Columns        []ColumnView
	Stream         []model.Article
	StreamSelected int
	Active         int
	Images         map[string]image.Image
	NewIDs         map[string]bool
	Loading        bool
	Paused         bool
	Help           bool
	CommandMode    bool
	CommandText    string
	OverlayTitle   string
	OverlayLines   []string
	Status         string
	Errors         []string
	LastRefresh    time.Time
	NextRefresh    time.Time
	Tick           int
}

// Render draws a true-color frame sized exactly to State.Width × State.Height.
func Render(state State) string { return render(state, true) }

// RenderPlain draws the same frame without escape sequences, useful for logs
// and snapshots.
func RenderPlain(state State) string { return render(state, false) }

func render(state State, colorEnabled bool) string {
	if state.Width <= 0 {
		state.Width = 120
	}
	if state.Height <= 0 {
		state.Height = 38
	}
	if state.Now.IsZero() {
		state.Now = time.Now()
	}
	if state.Config.Theme == "" {
		state.Config.Theme = "aurora"
	}
	theme := ThemeByName(state.Config.Theme)
	layout := ComputeLayout(state.Width, state.Height, len(state.Columns), state.Active, state.Config.Mode)

	lines := make([]string, 0, state.Height)
	lines = append(lines, renderHeader(state, layout, theme, colorEnabled)...)
	if len(state.OverlayLines) > 0 {
		lines = append(lines, renderOverlay(layout.Width, layout.ContentHeight, state.OverlayTitle, state.OverlayLines, theme, colorEnabled)...)
	} else if state.Help {
		lines = append(lines, renderHelp(layout.Width, layout.ContentHeight, theme, colorEnabled)...)
	} else if state.Config.Mode == "stream" {
		lines = append(lines, renderStream(state, layout, theme, colorEnabled)...)
	} else {
		lines = append(lines, renderDeck(state, layout, theme, colorEnabled)...)
	}
	lines = append(lines, renderFooter(state, layout, theme, colorEnabled)...)

	for len(lines) < state.Height {
		lines = append(lines, strings.Repeat(" ", state.Width))
	}
	if len(lines) > state.Height {
		lines = lines[:state.Height]
	}
	for i := range lines {
		lines[i] = FitANSI(lines[i], state.Width)
	}
	return strings.Join(lines, "\n")
}

func renderHeader(state State, layout Layout, theme Theme, colorEnabled bool) []string {
	mode := strings.ToUpper(state.Config.Mode)
	activity := "● LIVE"
	activityColor := theme.Success
	if state.Paused {
		activity = "Ⅱ PAUSED"
		activityColor = theme.Warning
	} else if state.Loading {
		activity = "◌ SYNC"
		activityColor = theme.Accent2
	}
	brand := styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent, "NEWSFALL"))
	left := " " + brand + styleFG(colorEnabled, theme.Faint, "  //  SIGNAL DESK")
	right := styleFG(colorEnabled, activityColor, activity) + styleFG(colorEnabled, theme.Muted, "  "+mode+"  "+state.Now.Format("15:04:05  MON 02 JAN")) + " "
	first := joinSides(left, right, layout.Width)
	if layout.HeaderHeight == 1 {
		return []string{first}
	}

	waveWidth := clampInt(layout.Width/3, 10, 42)
	wave := waveform(waveWidth, state.Tick)
	articleCount := len(state.Stream)
	if articleCount == 0 {
		for _, column := range state.Columns {
			articleCount += len(column.Articles)
		}
	}
	next := ""
	if state.Paused {
		next = "refresh held"
	} else if !state.NextRefresh.IsZero() {
		next = "next " + countdown(state.NextRefresh.Sub(state.Now))
	}
	secondLeft := " " + styleFG(colorEnabled, theme.Accent2, wave)
	secondRight := styleFG(colorEnabled, theme.Muted, fmt.Sprintf("%d signals  %s", articleCount, next)) + " "
	second := joinSides(secondLeft, secondRight, layout.Width)
	if layout.HeaderHeight == 2 {
		return []string{first, second}
	}
	rule := styleFG(colorEnabled, theme.Faint, strings.Repeat("─", layout.Width))
	return []string{first, second, rule}
}

func renderDeck(state State, layout Layout, theme Theme, colorEnabled bool) []string {
	if len(state.Columns) == 0 {
		return renderEmpty(layout.Width, layout.ContentHeight, "NO COLUMNS CONFIGURED", "Press : and run  column add <id> <title>", theme, colorEnabled)
	}
	start := clampInt(layout.StartColumn, 0, len(state.Columns)-1)
	end := start + layout.VisibleColumns
	if end > len(state.Columns) {
		end = len(state.Columns)
	}
	visible := state.Columns[start:end]
	if len(visible) == 0 {
		return renderEmpty(layout.Width, layout.ContentHeight, "NO VISIBLE COLUMNS", "Use h/l to move between desks", theme, colorEnabled)
	}
	widths := distributedWidths(layout.Width, len(visible), layout.Gap)
	columns := make([][]string, len(visible))
	for i, view := range visible {
		columns[i] = renderColumn(view, widths[i], layout.ContentHeight, start+i == state.Active, state, layout.CardHeight, theme, colorEnabled)
	}
	rows := make([]string, layout.ContentHeight)
	gap := strings.Repeat(" ", layout.Gap)
	for y := 0; y < layout.ContentHeight; y++ {
		var row strings.Builder
		for i := range columns {
			if i > 0 {
				row.WriteString(gap)
			}
			row.WriteString(columns[i][y])
		}
		rows[y] = FitANSI(row.String(), layout.Width)
	}
	return rows
}

func renderStream(state State, layout Layout, theme Theme, colorEnabled bool) []string {
	view := ColumnView{
		Column:   config.Column{ID: "stream", Title: "ALL SIGNALS // CHRONOLOGICAL", Accent: theme.Accent2},
		Articles: state.Stream, Selected: state.StreamSelected,
	}
	return renderColumn(view, layout.Width, layout.ContentHeight, true, state, layout.CardHeight, theme, colorEnabled)
}

func renderColumn(view ColumnView, width, height int, active bool, state State, preferredCardHeight int, theme Theme, colorEnabled bool) []string {
	if width < 1 || height < 1 {
		return []string{""}
	}
	headerHeight := 2
	if height < 7 {
		headerHeight = 1
	}
	accent := view.Column.Accent
	if accent == "" {
		accent = theme.Accent
	}
	position := "0/0"
	if len(view.Articles) > 0 {
		selected := normalizeIndex(view.Selected, len(view.Articles))
		position = fmt.Sprintf("%02d/%02d", selected+1, len(view.Articles))
	}
	marker := "◇"
	if active {
		marker = "◆"
	}
	label := " " + marker + " " + content.CleanText(view.Column.Title) + " "
	positionLabel := " " + position + " "
	fillWidth := width - VisibleWidth(label) - VisibleWidth(positionLabel) - 3
	if fillWidth < 0 {
		fillWidth = 0
	}
	top := styleFG(colorEnabled, accent, "╭─") + styleBold(colorEnabled, styleFG(colorEnabled, accent, label)) + styleFG(colorEnabled, theme.Faint, strings.Repeat("─", fillWidth)) + styleFG(colorEnabled, accent, positionLabel+"╮")
	lines := []string{FitANSI(top, width)}
	if headerHeight == 2 {
		sub := "│ " + columnSubline(view, state, width-4) + " │"
		lines = append(lines, FitANSI(styleFG(colorEnabled, theme.Muted, sub), width))
	}
	bodyHeight := height - headerHeight
	if bodyHeight <= 0 {
		return fitRows(lines, width, height)
	}
	if len(view.Articles) == 0 {
		empty := renderEmpty(width, bodyHeight, "QUIET CHANNEL", "Add a feed with :feed add …", theme, colorEnabled)
		lines = append(lines, empty...)
		return fitRows(lines, width, height)
	}

	slots := bodyHeight / maxInt(8, preferredCardHeight-2)
	if slots < 1 {
		slots = 1
	}
	if slots > 3 {
		slots = 3
	}
	if slots > len(view.Articles) {
		slots = len(view.Articles)
	}
	baseHeight := bodyHeight / slots
	remainder := bodyHeight % slots
	selected := normalizeIndex(view.Selected, len(view.Articles))
	for slot := 0; slot < slots; slot++ {
		cardHeight := baseHeight
		if slot < remainder {
			cardHeight++
		}
		index := (selected + slot) % len(view.Articles)
		isSelected := active && slot == 0
		lines = append(lines, renderCard(view.Articles[index], width, cardHeight, isSelected, state, theme, colorEnabled)...)
	}
	return fitRows(lines, width, height)
}

func columnSubline(view ColumnView, state State, width int) string {
	if width <= 0 {
		return ""
	}
	terms := view.Column.Include
	text := "assigned sources · ambient waterfall"
	if len(terms) > 0 {
		text = "tracking: " + strings.Join(terms, " · ")
	}
	if state.Paused {
		text = "paused · " + text
	}
	return FitANSI(text, width)
}

func renderCard(article model.Article, width, height int, selected bool, state State, theme Theme, colorEnabled bool) []string {
	if height <= 2 || width <= 4 {
		return renderCompactCard(article, width, height, selected, state, theme, colorEnabled)
	}
	borderColor := theme.Faint
	if selected {
		borderColor = theme.Accent2
	}
	source := content.CleanText(firstNonEmpty(article.Source, article.FeedName, "Unknown source"))
	age := ageLabel(state.Now, article.PublishedAt)
	badge := ""
	if state.NewIDs != nil && state.NewIDs[article.ID] {
		badge = " NEW ·"
	}
	header := badge + " " + source + " · " + age + " "
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	topFill := innerWidth - VisibleWidth(header)
	if topFill < 0 {
		topFill = 0
	}
	top := styleFG(colorEnabled, borderColor, "╭") + styleFG(colorEnabled, borderColor, "─") + styleBold(colorEnabled, styleFG(colorEnabled, func() string {
		if badge != "" {
			return theme.Warning
		}
		return theme.Muted
	}(), header)) + styleFG(colorEnabled, borderColor, strings.Repeat("─", topFillMinusOne(topFill))+"╮")
	top = FitANSI(top, width)

	innerHeight := height - 2
	reserveBottom := 2
	if innerHeight < 5 {
		reserveBottom = 1
	}
	visualHeight := innerHeight - reserveBottom
	if visualHeight < 1 {
		visualHeight = 1
	}
	artWidth := clampInt(width/3, 8, 16)
	if width >= 86 {
		artWidth = clampInt(width/4, 16, 26)
	}
	if artWidth+12 > innerWidth {
		artWidth = maxInt(0, innerWidth-12)
	}
	gap := 1
	textWidth := innerWidth - artWidth - gap
	if artWidth == 0 {
		gap = 0
		textWidth = innerWidth
	}

	art := articleArt(article, artWidth, visualHeight, state, theme, colorEnabled)
	title := content.CleanText(article.Title)
	excerpt := content.CleanText(article.Excerpt)
	titleLines := WrapText(title, textWidth, minInt(2, visualHeight))
	remaining := visualHeight - len(titleLines)
	textLines := append([]string(nil), titleLines...)
	if remaining > 0 && excerpt != "" {
		excerptLines := WrapText(excerpt, textWidth, remaining)
		textLines = append(textLines, excerptLines...)
	}
	for len(textLines) < visualHeight {
		textLines = append(textLines, "")
	}

	lines := []string{top}
	for row := 0; row < visualHeight; row++ {
		var body strings.Builder
		if artWidth > 0 {
			body.WriteString(FitANSI(art[row], artWidth))
			body.WriteString(" ")
		}
		text := textLines[row]
		if row < len(titleLines) {
			text = styleBold(colorEnabled, styleFG(colorEnabled, theme.Text, text))
		} else {
			text = styleFG(colorEnabled, theme.Muted, text)
		}
		body.WriteString(FitANSI(text, textWidth))
		lines = append(lines, boxBody(body.String(), innerWidth, borderColor, colorEnabled))
	}
	if reserveBottom >= 2 {
		lines = append(lines, boxBody(renderTags(article.Categories, innerWidth, theme, colorEnabled), innerWidth, borderColor, colorEnabled))
	}
	url := content.CleanText(article.URL)
	if colorEnabled && url != "" {
		url = Hyperlink(article.URL, styleFG(true, theme.Accent2, Underline(content.CleanText(article.URL))))
	}
	lines = append(lines, boxBody(url, innerWidth, borderColor, colorEnabled))
	bottom := styleFG(colorEnabled, borderColor, "╰"+strings.Repeat("─", innerWidth)+"╯")
	lines = append(lines, FitANSI(bottom, width))
	return fitRows(lines, width, height)
}

func renderCompactCard(article model.Article, width, height int, selected bool, state State, theme Theme, colorEnabled bool) []string {
	border := theme.Faint
	if selected {
		border = theme.Accent2
	}
	values := []string{content.CleanText(article.Title), firstNonEmpty(content.CleanText(article.Source), ageLabel(state.Now, article.PublishedAt)), content.CleanText(article.URL)}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		prefix := "│"
		if i == 0 {
			prefix = "╭"
		}
		if i == height-1 {
			prefix = "╰"
		}
		line := styleFG(colorEnabled, border, prefix) + " " + value
		lines = append(lines, FitANSI(line, width))
	}
	return lines
}

func articleArt(article model.Article, width, height int, state State, theme Theme, colorEnabled bool) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if state.Config.Images && state.Images != nil {
		if img := state.Images[article.ID]; img != nil {
			return ImageArt(img, width, height, theme, colorEnabled)
		}
	}
	seed := article.ID + "|" + article.Title + "|" + article.Source
	return FallbackArt(seed, width, height, theme, colorEnabled)
}

func renderTags(categories []string, width int, theme Theme, colorEnabled bool) string {
	if len(categories) == 0 {
		return styleFG(colorEnabled, theme.Faint, "# untagged")
	}
	var out strings.Builder
	for i, category := range categories {
		if i >= 3 {
			break
		}
		category = strings.ToUpper(content.CleanText(category))
		if category == "" {
			continue
		}
		chip := " " + category + " "
		if colorEnabled {
			out.WriteString(BG(theme.PanelAlt, styleFG(true, theme.Accent2, chip)))
		} else {
			out.WriteString("[" + category + "]")
		}
		out.WriteString(" ")
		if VisibleWidth(out.String()) >= width {
			break
		}
	}
	return out.String()
}

func boxBody(body string, innerWidth int, borderColor string, colorEnabled bool) string {
	return styleFG(colorEnabled, borderColor, "│") + FitANSI(body, innerWidth) + styleFG(colorEnabled, borderColor, "│")
}

func renderEmpty(width, height int, title, detail string, theme Theme, colorEnabled bool) []string {
	lines := make([]string, height)
	if height <= 0 {
		return lines
	}
	mid := height / 2
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	if mid >= 0 && mid < height {
		lines[mid] = center(styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent, title)), width)
	}
	if mid+1 < height {
		lines[mid+1] = center(styleFG(colorEnabled, theme.Muted, detail), width)
	}
	return lines
}

func renderOverlay(width, height int, title string, rows []string, theme Theme, colorEnabled bool) []string {
	if width < 28 || height < 5 {
		return renderEmpty(width, height, firstNonEmpty(title, "RESULT"), firstNonEmpty(rows...), theme, colorEnabled)
	}
	if title == "" {
		title = "COMMAND RESULT"
	}
	boxWidth := minInt(width, 104)
	inner := boxWidth - 2
	var body []string
	for _, row := range rows {
		wrapped := WrapText(content.CleanText(row), maxInt(1, inner-2), 3)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		body = append(body, wrapped...)
	}
	maxBody := maxInt(1, height-4)
	if len(body) > maxBody {
		body = body[:maxBody]
		if len(body) > 0 {
			body[len(body)-1] = FitANSI(body[len(body)-1]+" …", maxInt(1, inner-2))
		}
	}
	label := " " + content.CleanText(title) + " "
	fill := maxInt(0, inner-VisibleWidth(label)-1)
	box := []string{styleFG(colorEnabled, theme.Accent, "╭─") + styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent, label)) + styleFG(colorEnabled, theme.Faint, strings.Repeat("─", fill)+"╮")}
	for _, row := range body {
		box = append(box, boxBody(" "+styleFG(colorEnabled, theme.Text, row), inner, theme.Faint, colorEnabled))
	}
	box = append(box, boxBody(" "+styleFG(colorEnabled, theme.Muted, "esc dismisses this panel"), inner, theme.Faint, colorEnabled))
	box = append(box, styleFG(colorEnabled, theme.Faint, "╰"+strings.Repeat("─", inner)+"╯"))
	for i := range box {
		box[i] = FitANSI(box[i], boxWidth)
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	startY := maxInt(0, (height-len(box))/2)
	startX := maxInt(0, (width-boxWidth)/2)
	for i := 0; i < len(box) && startY+i < height; i++ {
		lines[startY+i] = strings.Repeat(" ", startX) + box[i] + strings.Repeat(" ", maxInt(0, width-startX-boxWidth))
	}
	return lines
}

func renderHelp(width, height int, theme Theme, colorEnabled bool) []string {
	if width < 28 || height < 8 {
		return renderEmpty(width, height, "HELP", "q quit · : command · ? close", theme, colorEnabled)
	}
	rows := []string{
		"KEYBOARD",
		"j / k / ↑ / ↓     move through stories",
		"h / l / ← / →     move between columns",
		"enter or o         open the selected article",
		"r refresh · p pause · a ambient · m mode · i images",
		"tab theme · : command · ? close help · q quit",
		"",
		"CONFIGURATION",
	}
	rows = append(rows, command.HelpLines()...)
	boxWidth := minInt(width, 88)
	inner := boxWidth - 2
	box := make([]string, 0, len(rows)+2)
	topLabel := " NEWSFALL CONTROL DECK "
	topFill := inner - VisibleWidth(topLabel)
	if topFill < 0 {
		topFill = 0
	}
	box = append(box, styleFG(colorEnabled, theme.Accent, "╭─")+styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent, topLabel))+styleFG(colorEnabled, theme.Faint, strings.Repeat("─", maxInt(0, topFill-1))+"╮"))
	for _, row := range rows {
		text := row
		if row == "KEYBOARD" || row == "CONFIGURATION" {
			text = styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent2, row))
		} else {
			text = styleFG(colorEnabled, theme.Text, row)
		}
		box = append(box, boxBody(" "+text, inner, theme.Faint, colorEnabled))
	}
	box = append(box, styleFG(colorEnabled, theme.Faint, "╰"+strings.Repeat("─", inner)+"╯"))
	for i := range box {
		box[i] = FitANSI(box[i], boxWidth)
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	startY := (height - len(box)) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (width - boxWidth) / 2
	for i := 0; i < len(box) && startY+i < height; i++ {
		left := strings.Repeat(" ", startX)
		rightWidth := width - startX - boxWidth
		if rightWidth < 0 {
			rightWidth = 0
		}
		lines[startY+i] = left + box[i] + strings.Repeat(" ", rightWidth)
	}
	return lines
}

func renderFooter(state State, layout Layout, theme Theme, colorEnabled bool) []string {
	if layout.FooterHeight <= 0 {
		return nil
	}
	status := state.Status
	if status == "" {
		status = "ready"
	}
	if len(state.Errors) > 0 {
		status += fmt.Sprintf(" · %d source errors", len(state.Errors))
	}
	left := " " + styleFG(colorEnabled, theme.Muted, status)
	right := ""
	if !state.LastRefresh.IsZero() {
		right = styleFG(colorEnabled, theme.Faint, "updated "+ageLabel(state.Now, state.LastRefresh)) + " "
	}
	statusLine := joinSides(left, right, layout.Width)
	if layout.FooterHeight == 1 {
		if state.CommandMode {
			return []string{renderCommandLine(state, layout.Width, theme, colorEnabled)}
		}
		return []string{statusLine}
	}
	var controlLine string
	if state.CommandMode {
		controlLine = renderCommandLine(state, layout.Width, theme, colorEnabled)
	} else {
		hints := " j/k scroll   h/l desk   ↵ open   m mode   r refresh   : command   ? help   q quit "
		controlLine = FitANSI(styleFG(colorEnabled, theme.Faint, hints), layout.Width)
	}
	return []string{statusLine, controlLine}
}

func renderCommandLine(state State, width int, theme Theme, colorEnabled bool) string {
	prompt := styleBold(colorEnabled, styleFG(colorEnabled, theme.Accent, " COMMAND ❯ "))
	input := content.CleanText(state.CommandText)
	cursor := styleFG(colorEnabled, theme.Accent2, "█")
	return FitANSI(prompt+styleFG(colorEnabled, theme.Text, input)+cursor, width)
}

func DemoState(width, height int) State {
	now := time.Now().Truncate(time.Second)
	cfg := config.Default()
	articles := []model.Article{
		{ID: "demo-ai-1", Title: "OPENAI SHIPS A FASTER CODING AGENT", URL: "https://example.com/openai-agent", Source: "Model Dispatch", FeedName: "Model Dispatch", Excerpt: "The new system coordinates long-running software work while keeping the operator in control.", Categories: []string{"AI", "Agents"}, ColumnHints: []string{"ai"}, PublishedAt: now.Add(-8 * time.Minute)},
		{ID: "demo-ai-2", Title: "Small models move onto the desktop", URL: "https://example.com/local-models", Source: "Silicon Brief", FeedName: "Silicon Brief", Excerpt: "New local inference tools turn ordinary workstations into useful private copilots.", Categories: []string{"AI", "Hardware"}, ColumnHints: []string{"ai"}, PublishedAt: now.Add(-41 * time.Minute)},
		{ID: "demo-ai-3", Title: "Researchers teach agents to verify their own work", URL: "https://example.com/agent-verification", Source: "Lab Notes", FeedName: "Lab Notes", Excerpt: "A practical evaluation loop catches more mistakes before output reaches a human reviewer.", Categories: []string{"Research"}, ColumnHints: []string{"ai"}, PublishedAt: now.Add(-2 * time.Hour)},
		{ID: "demo-car-1", Title: "BMW M3 TOURING MEETS AN EMPTY ALPINE ROAD", URL: "https://example.com/bmw-m3-touring", Source: "Apex Journal", FeedName: "Apex Journal", Excerpt: "A fast wagon, a wet mountain pass, and just enough restraint to make the long way home irresistible.", Categories: []string{"BMW", "Drive"}, ColumnHints: []string{"machines"}, PublishedAt: now.Add(-14 * time.Minute)},
		{ID: "demo-car-2", Title: "The analogue supercar market wakes up", URL: "https://example.com/analogue-market", Source: "Octane Wire", FeedName: "Octane Wire", Excerpt: "Manual gearboxes and compact dimensions are pulling a new generation toward once-overlooked cars.", Categories: []string{"Market", "Classics"}, ColumnHints: []string{"machines"}, PublishedAt: now.Add(-53 * time.Minute)},
		{ID: "demo-car-3", Title: "Inside a midnight endurance-racing pit stop", URL: "https://example.com/pit-stop", Source: "Grid Signal", FeedName: "Grid Signal", Excerpt: "Twenty-two seconds of choreography keeps a wounded prototype in the fight until sunrise.", Categories: []string{"Motorsport"}, ColumnHints: []string{"machines"}, PublishedAt: now.Add(-3 * time.Hour)},
		{ID: "demo-game-1", Title: "A tiny co-op game becomes the weekend obsession", URL: "https://example.com/coop", Source: "Checkpoint", FeedName: "Checkpoint", Excerpt: "Three players, one ridiculous objective, and the kind of emergent chaos that produces real stories.", Categories: []string{"Games", "Co-op"}, ColumnHints: []string{"games"}, PublishedAt: now.Add(-21 * time.Minute)},
		{ID: "demo-game-2", Title: "The return of weird, optimistic science fiction", URL: "https://example.com/scifi", Source: "Culture Loop", FeedName: "Culture Loop", Excerpt: "A wave of books and games is trading dystopian gray for strange worlds worth fighting to preserve.", Categories: []string{"Culture", "Sci-fi"}, ColumnHints: []string{"games"}, PublishedAt: now.Add(-76 * time.Minute)},
		{ID: "demo-game-3", Title: "Why tactile interfaces still feel like the future", URL: "https://example.com/interfaces", Source: "Object Lesson", FeedName: "Object Lesson", Excerpt: "Knobs, switches, CRT glow, and a little friction can make software feel more alive.", Categories: []string{"Design"}, ColumnHints: []string{"games"}, PublishedAt: now.Add(-4 * time.Hour)},
	}
	content.SortNewest(articles)
	assigned := content.Assign(articles, cfg.Columns, cfg.MaxPerColumn)
	views := make([]ColumnView, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		views = append(views, ColumnView{Column: column, Articles: assigned[column.ID]})
	}
	newIDs := map[string]bool{"demo-ai-1": true, "demo-car-1": true, "demo-game-1": true}
	return State{Width: width, Height: height, Now: now, Config: cfg, Columns: views, Stream: articles, Images: map[string]image.Image{}, NewIDs: newIDs, Active: 0, LastRefresh: now.Add(-9 * time.Second), NextRefresh: now.Add(4*time.Minute + 51*time.Second), Tick: 4, Status: "6 sources synchronized · offline demo signal"}
}

func waveform(width, tick int) string {
	levels := []rune("▁▂▃▄▅▆▇█▆▄▃▂")
	var out strings.Builder
	for i := 0; i < width; i++ {
		index := (i + tick) % len(levels)
		if index < 0 {
			index += len(levels)
		}
		out.WriteRune(levels[index])
	}
	return out.String()
}

func distributedWidths(total, count, gap int) []int {
	if count <= 0 {
		return nil
	}
	available := total - gap*(count-1)
	if available < count {
		available = count
	}
	base := available / count
	remainder := available % count
	out := make([]int, count)
	for i := range out {
		out[i] = base
		if i < remainder {
			out[i]++
		}
	}
	return out
}

func fitRows(lines []string, width, height int) []string {
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = FitANSI(lines[i], width)
	}
	return lines
}

func joinSides(left, right string, width int) string {
	gap := width - VisibleWidth(left) - VisibleWidth(right)
	if gap < 1 {
		return FitANSI(left+" "+right, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func center(value string, width int) string {
	visible := VisibleWidth(value)
	if visible >= width {
		return FitANSI(value, width)
	}
	left := (width - visible) / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", width-left-visible)
}

func styleFG(enabled bool, color, value string) string {
	if !enabled {
		return value
	}
	return FG(color, value)
}

func styleBold(enabled bool, value string) string {
	if !enabled {
		return value
	}
	return Bold(value)
}

func normalizeIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	index %= length
	if index < 0 {
		index += length
	}
	return index
}

func ageLabel(now, then time.Time) string {
	if then.IsZero() {
		return "now"
	}
	d := now.Sub(then)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return then.Format("02 Jan")
	}
}

func countdown(d time.Duration) string {
	if d <= 0 {
		return "due"
	}
	if d < time.Minute {
		return fmt.Sprintf("%02ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func topFillMinusOne(value int) int {
	if value > 0 {
		return value - 1
	}
	return 0
}

// Kept as a named helper to make the border geometry readable.
func topFillMinusOneIfPositive(value int) int { return topFillMinusOne(value) }

// Ensure deterministic iteration in future overlays that consume maps.
func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
