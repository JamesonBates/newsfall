package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"newsfall/internal/config"
)

func TestParseRecognizesSnapshotAndPathOptions(t *testing.T) {
	got, err := parse([]string{"--demo", "--snapshot", "--plain", "--width", "92", "--height", "24", "--config", "/tmp/cfg.json", "--data", "/tmp/data.json"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.Demo || !got.Snapshot || !got.Plain || got.Width != 92 || got.Height != 24 || got.ConfigPath != "/tmp/cfg.json" || got.DataPath != "/tmp/data.json" {
		t.Fatalf("options = %#v", got)
	}
}

func TestRunPrintsExactPlainDemoSnapshot(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--demo", "--snapshot", "--plain", "--width", "92", "--height", "24"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	output := strings.TrimSuffix(stdout.String(), "\n")
	if strings.Contains(output, "\x1b") || !strings.Contains(output, "NEWSFALL") || len(strings.Split(output, "\n")) != 24 {
		t.Fatalf("snapshot:\n%s", output)
	}
}

func TestRunAppliesNoninteractiveConfigurationCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Config{Columns: []config.Column{{ID: "ai", Title: "AI"}}}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", path, "--command", `feed add https://example.com/rss "Example Wire" ai`}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Feeds) != 1 || loaded.Feeds[0].Name != "Example Wire" || !strings.Contains(stdout.String(), "added source") {
		t.Fatalf("loaded=%#v stdout=%q", loaded, stdout.String())
	}
}

func TestRunPrintsVersionConfigPathAndHelpWithoutTerminal(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--config-path"}, {"--help"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v code=%d stderr=%s", args, code, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("%v produced no output", args)
		}
	}
}

func TestRunRejectsInvalidDimensionsAndConflictingModes(t *testing.T) {
	for _, args := range [][]string{{"--width", "-1", "--snapshot"}, {"--demo", "--command", "feed list"}, {"unexpected"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code == 0 {
			t.Fatalf("%v expected failure", args)
		}
	}
}
