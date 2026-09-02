package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"newsfall/internal/model"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "articles.json")
	want := []model.Article{{ID: "1", Title: "One", PublishedAt: time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC)}}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Title != "One" || !got[0].PublishedAt.Equal(want[0].PublishedAt) {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestLoadMissingIsEmptyAndCorruptIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := Load(path)
	if err != nil || got != nil {
		t.Fatalf("missing Load = %#v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected corrupt cache error")
	}
}
