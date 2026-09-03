package app

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"newsfall/internal/config"
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

func TestAcceptFetchSurfacesErrorDetailsAndPersistsDiscoveredFeedURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	dataPath := filepath.Join(dir, "articles.json")
	cfg := config.Config{
		Columns: []config.Column{{ID: "ai", Title: "AI"}},
		Feeds:   []config.Feed{{Name: "Example", URL: "https://example.com", Columns: []string{"ai"}}},
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	controller := NewController(cfg, nil)
	interaction := NewInteraction(controller)
	r := runner{
		runtimeModel: runtimeModel{interaction: interaction, images: map[string]image.Image{}, loading: true},
		configPath:   configPath,
		dataPath:     dataPath,
	}
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	result := feed.Result{
		Articles: []model.Article{{ID: "one", Title: "One", URL: "https://example.com/one", ColumnHints: []string{"ai"}, PublishedAt: now}},
		Errors:   []feed.FeedError{{Feed: "Other", URL: "https://other.example", Err: errString("HTTP 403 Forbidden")}},
		Resolutions: []feed.Resolution{{
			Feed: "Example", From: "https://example.com", To: "https://example.com/rss.xml",
		}},
	}
	r.acceptFetch(result, now)

	if got := r.interaction.Controller.Config.Feeds[0].URL; got != "https://example.com/rss.xml" {
		t.Fatalf("resolved URL = %q", got)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Feeds[0].URL; got != "https://example.com/rss.xml" {
		t.Fatalf("saved resolved URL = %q", got)
	}
	if len(r.interaction.SourceErrors) != 1 || !strings.Contains(r.interaction.Status, "Other") || !strings.Contains(r.interaction.Status, "press e") {
		t.Fatalf("status=%q errors=%#v", r.interaction.Status, r.interaction.SourceErrors)
	}
}

func TestRefreshRequestedDuringSynchronizationIsQueuedAndRunsNext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Queued</title><item><guid>one</guid><title>Queued story</title><link>/story</link></item></channel></rss>`))
	}))
	defer server.Close()

	cfg := config.Config{
		Columns: []config.Column{{ID: "ai", Title: "AI"}},
		Feeds:   []config.Feed{{Name: "Queued", URL: server.URL, Columns: []string{"ai"}}},
	}
	if err := config.Validate(&cfg); err != nil {
		t.Fatal(err)
	}
	r := runner{
		runtimeModel: runtimeModel{interaction: NewInteraction(NewController(cfg, nil)), images: map[string]image.Image{}, loading: true},
		ctx:          context.Background(),
		configPath:   filepath.Join(t.TempDir(), "config.json"),
		dataPath:     filepath.Join(t.TempDir(), "articles.json"),
		fetcher:      feed.NewFetcher(),
		fetchResults: make(chan feed.Result, 1),
	}
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	r.requestRefresh(now)
	if !r.refreshQueued {
		t.Fatal("refresh was discarded instead of queued")
	}

	r.acceptFetch(feed.Result{}, now)
	if r.refreshQueued || !r.loading {
		t.Fatalf("queued refresh did not start: queued=%v loading=%v", r.refreshQueued, r.loading)
	}
	select {
	case result := <-r.fetchResults:
		if len(result.Errors) != 0 || len(result.Articles) != 1 {
			t.Fatalf("queued fetch result = %#v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued refresh did not execute")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
