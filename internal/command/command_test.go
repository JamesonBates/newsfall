package command

import (
	"reflect"
	"strings"
	"testing"

	"newsfall/internal/config"
)

func TestParseSupportsColonQuotesAndEscapes(t *testing.T) {
	got, err := Parse(`:feed add "https://example.com/rss" "Example Wire" ai`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Command{Name: "feed", Args: []string{"add", "https://example.com/rss", "Example Wire", "ai"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse = %#v, want %#v", got, want)
	}

	got, err = Parse(`topic add ai machine\ learning`)
	if err != nil {
		t.Fatalf("Parse escaped space: %v", err)
	}
	if got.Args[2] != "machine learning" {
		t.Fatalf("escaped argument = %q", got.Args[2])
	}
}

func TestParseRejectsEmptyAndUnclosedQuotes(t *testing.T) {
	for _, input := range []string{" ", `feed add "broken`} {
		if _, err := Parse(input); err == nil {
			t.Fatalf("Parse(%q) expected error", input)
		}
	}
}

func TestApplyAddsAndRemovesFeedWithoutMutatingInput(t *testing.T) {
	original := config.Config{Columns: []config.Column{{ID: "ai", Title: "AI"}}}
	if err := config.Validate(&original); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	got, effect, err := Execute(original, `feed add https://example.com/rss "Example Wire" ai`)
	if err != nil {
		t.Fatalf("add feed: %v", err)
	}
	if len(original.Feeds) != 0 {
		t.Fatalf("input mutated: %#v", original.Feeds)
	}
	if len(got.Feeds) != 1 || got.Feeds[0].Name != "Example Wire" || got.Feeds[0].Columns[0] != "ai" {
		t.Fatalf("feed = %#v", got.Feeds)
	}
	if !effect.Save || !effect.Refresh || !strings.Contains(effect.Message, "Example Wire") {
		t.Fatalf("effect = %#v", effect)
	}

	got, effect, err = Execute(got, `feed remove "Example Wire"`)
	if err != nil {
		t.Fatalf("remove feed: %v", err)
	}
	if len(got.Feeds) != 0 || !effect.Save || !effect.Refresh {
		t.Fatalf("remove = %#v %#v", got.Feeds, effect)
	}
}

func TestApplyManagesColumnsAndTopics(t *testing.T) {
	cfg := config.Config{}
	cfg, _, err := Execute(cfg, `column add science "SCIENCE DESK" space climate`)
	if err != nil {
		t.Fatalf("column add: %v", err)
	}
	if len(cfg.Columns) != 1 || !reflect.DeepEqual(cfg.Columns[0].Include, []string{"space", "climate"}) {
		t.Fatalf("column = %#v", cfg.Columns)
	}
	cfg, _, err = Execute(cfg, `topic add science "machine learning"`)
	if err != nil {
		t.Fatalf("topic add: %v", err)
	}
	if got := cfg.Columns[0].Include; !reflect.DeepEqual(got, []string{"space", "climate", "machine learning"}) {
		t.Fatalf("topics = %#v", got)
	}
	cfg, _, err = Execute(cfg, `topic remove science CLIMATE`)
	if err != nil {
		t.Fatalf("topic remove: %v", err)
	}
	if got := cfg.Columns[0].Include; !reflect.DeepEqual(got, []string{"space", "machine learning"}) {
		t.Fatalf("topics = %#v", got)
	}
	cfg, _, err = Execute(cfg, `column remove science`)
	if err != nil {
		t.Fatalf("column remove: %v", err)
	}
	if len(cfg.Columns) != 0 {
		t.Fatalf("columns = %#v", cfg.Columns)
	}
}

func TestApplySettingsAndImmediateEffects(t *testing.T) {
	cfg := config.Default()
	steps := []struct {
		line  string
		check func(config.Config, Effect) bool
	}{
		{"refresh 90s", func(c config.Config, e Effect) bool { return c.Refresh == "1m30s" && e.Save }},
		{"drift 8s", func(c config.Config, e Effect) bool { return c.Drift == "8s" && e.Save }},
		{"theme ocean", func(c config.Config, e Effect) bool { return c.Theme == "ocean" && e.Save }},
		{"mode stream", func(c config.Config, e Effect) bool { return c.Mode == "stream" && e.Save }},
		{"images off", func(c config.Config, e Effect) bool { return !c.Images && e.Save }},
		{"ambient on", func(c config.Config, e Effect) bool { return c.Ambient && e.Save }},
		{"refresh now", func(c config.Config, e Effect) bool { return e.Refresh && !e.Save }},
		{"reload", func(c config.Config, e Effect) bool { return e.Reload && !e.Save }},
	}
	var err error
	for _, step := range steps {
		var effect Effect
		cfg, effect, err = Execute(cfg, step.line)
		if err != nil {
			t.Fatalf("%s: %v", step.line, err)
		}
		if !step.check(cfg, effect) {
			t.Fatalf("%s => %#v %#v", step.line, cfg, effect)
		}
	}
}

func TestApplyListsAndReportsActionableErrors(t *testing.T) {
	cfg := config.Default()
	_, effect, err := Execute(cfg, "feed list")
	if err != nil || len(effect.Output) != len(cfg.Feeds) {
		t.Fatalf("feed list: %#v %v", effect, err)
	}
	_, effect, err = Execute(cfg, "column list")
	if err != nil || len(effect.Output) != len(cfg.Columns) {
		t.Fatalf("column list: %#v %v", effect, err)
	}
	for _, line := range []string{
		"feed add not-a-url bad ai",
		"feed add https://example.com/rss bad missing",
		"topic add missing ai",
		"theme neon",
		"refresh 2s",
		"images perhaps",
		"unknown thing",
	} {
		if _, _, err := Execute(cfg, line); err == nil {
			t.Fatalf("%q expected error", line)
		}
	}
}
