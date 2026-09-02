package app

import (
	"image"
	"strings"
	"testing"
	"time"

	"newsfall/internal/feed"
	"newsfall/internal/model"
)

func TestRuntimeModelBuildsCompleteUIView(t *testing.T) {
	controller := controllerFixture()
	interaction := NewInteraction(controller)
	interaction.Paused = true
	interaction.CommandMode = true
	interaction.CommandText = "feed list"
	interaction.OverlayTitle = "FEED LIST"
	interaction.OverlayLines = []string{"one"}
	now := time.Date(2026, 9, 2, 18, 0, 0, 0, time.UTC)
	model := runtimeModel{
		interaction: interaction,
		width:       130, height: 38, images: map[string]image.Image{},
		loading: true, errors: []string{"bad feed"}, tick: 9,
		lastRefresh: now.Add(-time.Minute), nextRefresh: now.Add(time.Minute),
	}
	state := model.view(now)
	if state.Width != 130 || state.Height != 38 || !state.Loading || !state.Paused || state.CommandText != "feed list" {
		t.Fatalf("state = %#v", state)
	}
	if state.OverlayTitle != "FEED LIST" || len(state.Errors) != 1 || state.Tick != 9 {
		t.Fatalf("overlay/status = %#v", state)
	}
}

func TestImageCandidatesPrioritizeSelectionsAndSkipMissingOrDuplicateURLs(t *testing.T) {
	controller := controllerFixture()
	controller.Articles[0].ImageURL = "https://example.com/a.png"
	controller.Articles[1].ImageURL = "https://example.com/b.png"
	controller.Articles[2].ImageURL = "https://example.com/c.png"
	controller.Articles = append(controller.Articles, model.Article{ID: "no-image", Title: "No image"})
	controller.rebuild("", nil, "")
	controller.MoveVertical(1) // ai-old should be first candidate.
	got := imageCandidates(controller, 3)
	if len(got) != 3 || got[0].ID != "ai-old" {
		t.Fatalf("candidates = %#v", got)
	}
	seen := map[string]bool{}
	for _, article := range got {
		if article.ImageURL == "" || seen[article.ID] {
			t.Fatalf("invalid candidate: %#v", got)
		}
		seen[article.ID] = true
	}
}

func TestFetchStatusDescribesPartialFailureAndNewStories(t *testing.T) {
	status := fetchStatus(6, feed.Result{Articles: []model.Article{{ID: "1"}, {ID: "2"}}, Errors: []feed.FeedError{{Feed: "Bad"}}}, 2)
	for _, part := range []string{"5/6", "2 stories", "2 new", "1 error"} {
		if !strings.Contains(status, part) {
			t.Fatalf("status %q missing %q", status, part)
		}
	}
}
