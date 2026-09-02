package app

import (
	"testing"
	"time"

	"newsfall/internal/config"
	"newsfall/internal/model"
)

func controllerFixture() *Controller {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	cfg := config.Config{
		Mode: "deck", Theme: "aurora", Refresh: "5m", Drift: "12s", MaxItems: 10, MaxPerColumn: 10,
		Columns: []config.Column{{ID: "ai", Title: "AI"}, {ID: "cars", Title: "CARS"}},
	}
	_ = config.Validate(&cfg)
	articles := []model.Article{
		{ID: "ai-new", Title: "AI new", ColumnHints: []string{"ai"}, PublishedAt: now},
		{ID: "ai-old", Title: "AI old", ColumnHints: []string{"ai"}, PublishedAt: now.Add(-time.Hour)},
		{ID: "car-new", Title: "Car new", URL: "https://example.com/car", ColumnHints: []string{"cars"}, PublishedAt: now.Add(-time.Minute)},
	}
	return NewController(cfg, articles)
}

func TestControllerNavigatesDeckAndStreamAndSelectsArticle(t *testing.T) {
	controller := controllerFixture()
	controller.MoveVertical(1)
	article, ok := controller.SelectedArticle()
	if !ok || article.ID != "ai-old" {
		t.Fatalf("selected = %#v, %v", article, ok)
	}
	controller.MoveHorizontal(1)
	article, ok = controller.SelectedArticle()
	if !ok || article.ID != "car-new" {
		t.Fatalf("selected car = %#v, %v", article, ok)
	}
	controller.Config.Mode = "stream"
	controller.MoveVertical(2)
	article, ok = controller.SelectedArticle()
	if !ok || article.ID != "ai-old" {
		t.Fatalf("stream selected = %#v, %v", article, ok)
	}
	controller.First()
	article, _ = controller.SelectedArticle()
	if article.ID != "ai-new" {
		t.Fatalf("first = %s", article.ID)
	}
	controller.Last()
	article, _ = controller.SelectedArticle()
	if article.ID != "ai-old" {
		t.Fatalf("last = %s", article.ID)
	}
}

func TestControllerMergeMarksNewStoriesLimitsCacheAndPreservesSelection(t *testing.T) {
	controller := controllerFixture()
	controller.MoveVertical(1)
	now := time.Date(2026, 9, 2, 13, 0, 0, 0, time.UTC)
	added := controller.Merge([]model.Article{
		{ID: "brand-new", Title: "Breaking AI", ColumnHints: []string{"ai"}, PublishedAt: now},
		{ID: "ai-new", Title: "AI new updated", ColumnHints: []string{"ai"}, PublishedAt: now.Add(-time.Minute)},
	})
	if added != 1 || !controller.NewIDs["brand-new"] {
		t.Fatalf("new = %d ids=%#v", added, controller.NewIDs)
	}
	article, _ := controller.SelectedArticle()
	if article.ID != "ai-old" {
		t.Fatalf("selection was not preserved: %s", article.ID)
	}
	if len(controller.Articles) > controller.Config.MaxItems {
		t.Fatalf("cache limit ignored: %d", len(controller.Articles))
	}
}

func TestControllerDriftAdvancesEveryColumnAndCyclesTheme(t *testing.T) {
	controller := controllerFixture()
	controller.Drift()
	if controller.Columns[0].Selected != 1 || controller.Columns[1].Selected != 0 || controller.StreamSelected != 1 {
		t.Fatalf("drift = %#v stream=%d", controller.Columns, controller.StreamSelected)
	}
	controller.CycleTheme()
	if controller.Config.Theme != "ember" {
		t.Fatalf("theme = %s", controller.Config.Theme)
	}
	controller.CycleTheme()
	controller.CycleTheme()
	controller.CycleTheme()
	if controller.Config.Theme != "aurora" {
		t.Fatalf("theme wrap = %s", controller.Config.Theme)
	}
}

func TestControllerRebuildPreservesColumnByIDAcrossConfigChanges(t *testing.T) {
	controller := controllerFixture()
	controller.Active = 1
	next := controller.Config
	next.Columns = []config.Column{{ID: "cars", Title: "AUTOS"}, {ID: "ai", Title: "AI"}, {ID: "all", Title: "ALL"}}
	if err := config.Validate(&next); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	controller.SetConfig(next)
	if controller.Columns[controller.Active].Column.ID != "cars" {
		t.Fatalf("active column lost: active=%d columns=%#v", controller.Active, controller.Columns)
	}
}

func TestControllerSetConfigAppliesNewLimitsBeforeReassignment(t *testing.T) {
	controller := controllerFixture()
	next := controller.Config
	next.MaxItems = 1
	next.MaxPerColumn = 10
	if err := config.Validate(&next); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	controller.SetConfig(next)
	if len(controller.Articles) != 1 {
		t.Fatalf("articles = %d", len(controller.Articles))
	}
	assigned := 0
	for _, view := range controller.Columns {
		assigned += len(view.Articles)
	}
	if assigned != 1 {
		t.Fatalf("columns were built before the new item limit: %#v", controller.Columns)
	}
}
