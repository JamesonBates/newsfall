package content

import (
	"testing"
	"time"

	"newsfall/internal/config"
	"newsfall/internal/model"
)

func TestDeduplicateKeepsNewestAndMergesUsefulMetadata(t *testing.T) {
	old := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	items := []model.Article{
		{ID: "same", Title: "Old", URL: "https://e/x", Excerpt: "has excerpt", Categories: []string{"AI"}, ColumnHints: []string{"ai"}, PublishedAt: old},
		{ID: "same", Title: "New", URL: "https://e/x", ImageURL: "https://e/i.jpg", Categories: []string{"Research"}, ColumnHints: []string{"tech"}, PublishedAt: newer},
	}
	got := Deduplicate(items)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Title != "New" || got[0].Excerpt != "has excerpt" || got[0].ImageURL == "" {
		t.Fatalf("merged article = %#v", got[0])
	}
	if len(got[0].Categories) != 2 || len(got[0].ColumnHints) != 2 {
		t.Fatalf("metadata not merged: %#v", got[0])
	}
}

func TestMatchesUsesHintsTermsAndExclusions(t *testing.T) {
	article := model.Article{
		Title:       "OpenAI launches a new coding agent",
		Excerpt:     "A model for software work",
		Source:      "Example",
		Categories:  []string{"AI"},
		ColumnHints: []string{"ai"},
	}
	if !Matches(article, config.Column{ID: "ai", Include: []string{"openai"}}) {
		t.Fatal("expected AI match")
	}
	if Matches(article, config.Column{ID: "cars", Include: []string{"openai"}}) {
		t.Fatal("column hint should reject another column")
	}
	if Matches(article, config.Column{ID: "ai", Include: []string{"openai"}, Exclude: []string{"coding"}}) {
		t.Fatal("exclusion should override inclusion")
	}
	withoutHints := article
	withoutHints.ColumnHints = nil
	if !Matches(withoutHints, config.Column{ID: "anything"}) {
		t.Fatal("empty column rules should act as all-items")
	}
}

func TestAssignLimitsEachColumnAndPreservesNewestOrder(t *testing.T) {
	now := time.Now()
	articles := []model.Article{
		{ID: "1", Title: "AI one", ColumnHints: []string{"ai"}, PublishedAt: now},
		{ID: "2", Title: "AI two", ColumnHints: []string{"ai"}, PublishedAt: now.Add(time.Minute)},
		{ID: "3", Title: "Car", ColumnHints: []string{"cars"}, PublishedAt: now.Add(2 * time.Minute)},
	}
	columns := []config.Column{{ID: "ai", Title: "AI"}, {ID: "cars", Title: "Cars"}}
	got := Assign(articles, columns, 1)
	if len(got["ai"]) != 1 || got["ai"][0].ID != "2" {
		t.Fatalf("AI assignment = %#v", got["ai"])
	}
	if len(got["cars"]) != 1 || got["cars"][0].ID != "3" {
		t.Fatalf("car assignment = %#v", got["cars"])
	}
}
