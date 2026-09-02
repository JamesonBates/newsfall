package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultIsValidAndPersonalized(t *testing.T) {
	cfg := Default()
	if err := Validate(&cfg); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	want := []string{"AI + TECH", "MACHINES", "GAMES + CULTURE"}
	if len(cfg.Columns) != len(want) {
		t.Fatalf("columns = %d, want %d", len(cfg.Columns), len(want))
	}
	for i := range want {
		if cfg.Columns[i].Title != want[i] {
			t.Errorf("column %d title = %q, want %q", i, cfg.Columns[i].Title, want[i])
		}
	}
	if len(cfg.Feeds) < 3 {
		t.Fatalf("starter feeds = %d, want at least 3", len(cfg.Feeds))
	}
	if got := RefreshDuration(cfg); got != 5*time.Minute {
		t.Fatalf("refresh = %s", got)
	}
	if got := DriftDuration(cfg); got != 12*time.Second {
		t.Fatalf("drift = %s", got)
	}
}

func TestValidateNormalizesIDsNamesAndDefaults(t *testing.T) {
	cfg := Config{
		Columns: []Column{{Title: "My Curious Feed"}},
		Feeds:   []Feed{{URL: "https://www.Example.com/feed.xml"}},
	}
	if err := Validate(&cfg); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Columns[0].ID != "my-curious-feed" {
		t.Fatalf("column id = %q", cfg.Columns[0].ID)
	}
	if cfg.Feeds[0].Name != "example.com" {
		t.Fatalf("feed name = %q", cfg.Feeds[0].Name)
	}
	if cfg.Mode != "deck" || cfg.Theme != "aurora" || cfg.Refresh != "5m" || cfg.Drift != "12s" {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
	if cfg.MaxItems != 240 || cfg.MaxPerColumn != 80 {
		t.Fatalf("item limits not defaulted: %#v", cfg)
	}
}

func TestValidateRejectsDuplicatesAndUnknownColumn(t *testing.T) {
	tests := []Config{
		{
			Columns: []Column{{ID: "all", Title: "All"}},
			Feeds: []Feed{
				{Name: "A", URL: "https://example.com/feed"},
				{Name: "B", URL: "https://example.com/feed/"},
			},
		},
		{Columns: []Column{{ID: "ai", Title: "AI"}, {ID: "AI", Title: "Again"}}},
		{
			Columns: []Column{{ID: "ai", Title: "AI"}},
			Feeds:   []Feed{{Name: "A", URL: "https://example.com/rss", Columns: []string{"missing"}}},
		},
	}
	for i := range tests {
		if err := Validate(&tests[i]); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

func TestValidateRequiresOneToThreeColumns(t *testing.T) {
	tests := []Config{
		{},
		{Columns: []Column{{ID: "one"}, {ID: "two"}, {ID: "three"}, {ID: "four"}}},
	}
	for i := range tests {
		if err := Validate(&tests[i]); err == nil {
			t.Fatalf("case %d: expected column-count validation error", i)
		}
	}
}

func TestSaveLoadRoundTripAndMissingDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(cfg.Columns) != 3 {
		t.Fatalf("default columns = %d", len(cfg.Columns))
	}
	cfg.Theme = "ember"
	cfg.Images = false
	cfg.Ambient = false
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "ember" || got.Images || got.Ambient {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
