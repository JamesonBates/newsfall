package ui

import (
	"image"
	"strings"
	"testing"
	"time"

	"newsfall/internal/config"
	"newsfall/internal/model"
)

func sampleState(width, height int) State {
	now := time.Date(2026, 9, 2, 21, 42, 0, 0, time.UTC)
	cfg := config.Default()
	article := model.Article{
		ID: "story-1", Title: "A vivid terminal feed arrives", URL: "https://example.com/story",
		Source: "Signal Wire", FeedName: "Signal Wire", Excerpt: "A colorful, calm stream for the terminal with useful details and a little motion.",
		Categories: []string{"Design", "Technology"}, PublishedAt: now.Add(-17 * time.Minute),
	}
	columns := make([]ColumnView, 0, len(cfg.Columns))
	for _, column := range cfg.Columns {
		columns = append(columns, ColumnView{Column: column, Articles: []model.Article{article}, Selected: 0})
	}
	return State{
		Width: width, Height: height, Now: now, Config: cfg, Columns: columns,
		Stream: []model.Article{article}, Active: 0, NewIDs: map[string]bool{"story-1": true},
		Images: map[string]image.Image{}, LastRefresh: now.Add(-12 * time.Second), NextRefresh: now.Add(4*time.Minute + 48*time.Second), Tick: 3,
		Status: "3 sources synchronized",
	}
}

func TestRenderPlainDeckHasExactViewportAndRequiredMetadata(t *testing.T) {
	state := sampleState(150, 40)
	got := RenderPlain(state)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("plain render contains escape sequences: %q", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != state.Height {
		t.Fatalf("height = %d, want %d\n%s", len(lines), state.Height, got)
	}
	for i, line := range lines {
		if VisibleWidth(line) != state.Width {
			t.Fatalf("line %d width = %d, want %d", i, VisibleWidth(line), state.Width)
		}
	}
	for _, required := range []string{"NEWSFALL", "AI + TECH", "MACHINES", "GAMES + CULTURE", "A vivid terminal", "Signal Wire", "17m", "https://example.com/story"} {
		if !strings.Contains(got, required) {
			t.Errorf("missing %q\n%s", required, got)
		}
	}
}

func TestRenderStreamTinyHelpAndCommandModesDoNotLeakRemoteEscapes(t *testing.T) {
	state := sampleState(64, 22)
	state.Config.Mode = "stream"
	state.Stream[0].Title = "hostile\x1b[2J title"
	state.CommandMode = true
	state.CommandText = `feed add "https://example.com/rss"`
	got := Render(state)
	plain := StripANSI(got)
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "COMMAND") {
		t.Fatalf("unsafe or missing command mode: %q", plain)
	}

	state.CommandMode = false
	state.Help = true
	plain = RenderPlain(state)
	if !strings.Contains(plain, "KEYBOARD") || !strings.Contains(plain, "CONFIGURATION") {
		t.Fatalf("help overlay missing:\n%s", plain)
	}
}

func TestDemoStateProducesAUsefulOfflineSnapshot(t *testing.T) {
	state := DemoState(132, 38)
	got := RenderPlain(state)
	if !strings.Contains(got, "NEWSFALL") || !strings.Contains(got, "OPENAI") || !strings.Contains(got, "BMW") {
		t.Fatalf("demo snapshot is not representative:\n%s", got)
	}
}

func TestRenderOverlayShowsCommandResults(t *testing.T) {
	state := sampleState(92, 28)
	state.OverlayTitle = "CONFIGURED FEEDS"
	state.OverlayLines = []string{"Hacker News  https://hnrss.org/frontpage", "BMWBLOG  https://www.bmwblog.com/feed/"}
	plain := RenderPlain(state)
	if !strings.Contains(plain, "CONFIGURED FEEDS") || !strings.Contains(plain, "Hacker News") || !strings.Contains(plain, "BMWBLOG") {
		t.Fatalf("overlay missing:\n%s", plain)
	}
}

func TestRenderEveryResponsiveBreakpointHasExactGeometry(t *testing.T) {
	widths := []int{1, 20, 55, 56, 85, 86, 131, 132, 180}
	heights := []int{1, 5, 11, 17, 18, 40}
	modes := []string{"deck", "stream"}
	themes := []string{"aurora", "ember", "ocean", "mono"}
	for _, width := range widths {
		for _, height := range heights {
			for _, mode := range modes {
				for _, theme := range themes {
					state := sampleState(width, height)
					state.Config.Mode = mode
					state.Config.Theme = theme
					got := RenderPlain(state)
					lines := strings.Split(got, "\n")
					if len(lines) != height {
						t.Fatalf("%dx%d %s/%s height = %d", width, height, mode, theme, len(lines))
					}
					for row, line := range lines {
						if gotWidth := VisibleWidth(line); gotWidth != width {
							t.Fatalf("%dx%d %s/%s row %d width = %d", width, height, mode, theme, row, gotWidth)
						}
					}
				}
			}
		}
	}
}

func TestHeaderShowsTheActualUppercaseWeekdayAndMonth(t *testing.T) {
	state := sampleState(120, 30)
	state.Now = time.Date(2026, 9, 3, 9, 53, 29, 0, time.Local)
	plain := RenderPlain(state)
	if !strings.Contains(plain, "09:53:29  THU 03 SEP") {
		t.Fatalf("header date is not formatted from the current time:\n%s", strings.Split(plain, "\n")[0])
	}
}
