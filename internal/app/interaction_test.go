package app

import (
	"testing"
)

func TestInteractionHandlesNavigationOpeningAndQuickToggles(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	interaction.Handle(Key{Kind: KeyRune, Rune: 'j'})
	article, _ := interaction.Controller.SelectedArticle()
	if article.ID != "ai-old" {
		t.Fatalf("j selected %s", article.ID)
	}
	interaction.Handle(Key{Kind: KeyRune, Rune: 'l'})
	outcome := interaction.Handle(Key{Kind: KeyEnter})
	if outcome.OpenURL != "https://example.com/car" {
		t.Fatalf("open = %#v", outcome)
	}
	outcome = interaction.Handle(Key{Kind: KeyRune, Rune: 'm'})
	if !outcome.SaveConfig || interaction.Controller.Config.Mode != "stream" {
		t.Fatalf("mode = %#v %s", outcome, interaction.Controller.Config.Mode)
	}
	outcome = interaction.Handle(Key{Kind: KeyTab})
	if !outcome.SaveConfig || interaction.Controller.Config.Theme != "ember" {
		t.Fatalf("theme = %#v %s", outcome, interaction.Controller.Config.Theme)
	}
	outcome = interaction.Handle(Key{Kind: KeyRune, Rune: 'p'})
	if outcome.SaveConfig || !interaction.Paused {
		t.Fatalf("pause = %#v paused=%v", outcome, interaction.Paused)
	}
	outcome = interaction.Handle(Key{Kind: KeyRune, Rune: 'r'})
	if !outcome.Refresh {
		t.Fatalf("refresh = %#v", outcome)
	}
}

func TestInteractionCommandModeAppliesQuotedConfigCommand(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	interaction.Handle(Key{Kind: KeyRune, Rune: ':'})
	for _, r := range `feed add https://example.com/rss "Example Wire" ai` {
		outcome := interaction.Handle(Key{Kind: KeyRune, Rune: r})
		if outcome.Quit {
			t.Fatal("typing q in a command must not quit")
		}
	}
	outcome := interaction.Handle(Key{Kind: KeyEnter})
	if !outcome.SaveConfig || !outcome.Refresh || interaction.CommandMode {
		t.Fatalf("outcome = %#v command=%v", outcome, interaction.CommandMode)
	}
	feeds := interaction.Controller.Config.Feeds
	if len(feeds) != 1 || feeds[0].Name != "Example Wire" {
		t.Fatalf("feeds = %#v", feeds)
	}
}

func TestInteractionShowsListResultsAndActionableCommandErrors(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	interaction.CommandMode = true
	interaction.CommandText = "column list"
	outcome := interaction.Handle(Key{Kind: KeyEnter})
	if outcome.SaveConfig || interaction.OverlayTitle == "" || len(interaction.OverlayLines) != 2 {
		t.Fatalf("list = %#v title=%q lines=%#v", outcome, interaction.OverlayTitle, interaction.OverlayLines)
	}
	interaction.Handle(Key{Kind: KeyEscape})
	if len(interaction.OverlayLines) != 0 {
		t.Fatal("escape should dismiss overlay")
	}
	interaction.CommandMode = true
	interaction.CommandText = "theme radioactive"
	outcome = interaction.Handle(Key{Kind: KeyEnter})
	if outcome.SaveConfig || interaction.Status == "" || interaction.OverlayTitle != "COMMAND ERROR" || len(interaction.OverlayLines) == 0 {
		t.Fatalf("invalid command = %#v status=%q title=%q lines=%#v", outcome, interaction.Status, interaction.OverlayTitle, interaction.OverlayLines)
	}
}

func TestInteractionShowsSourceErrorsFromKeyOrCommand(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	interaction.SourceErrors = []string{
		"Example: webpage has no discoverable RSS, Atom, or JSON Feed",
		"Other: HTTP 403 Forbidden",
	}

	interaction.Handle(Key{Kind: KeyRune, Rune: 'e'})
	if interaction.OverlayTitle != "SOURCE ERRORS" || len(interaction.OverlayLines) != 2 {
		t.Fatalf("error overlay = %q %#v", interaction.OverlayTitle, interaction.OverlayLines)
	}

	interaction.Handle(Key{Kind: KeyEscape})
	interaction.CommandMode = true
	interaction.CommandText = "feed errors"
	outcome := interaction.Handle(Key{Kind: KeyEnter})
	if outcome.SaveConfig || outcome.Refresh || interaction.OverlayTitle != "SOURCE ERRORS" || len(interaction.OverlayLines) != 2 {
		t.Fatalf("feed errors = %#v title=%q lines=%#v", outcome, interaction.OverlayTitle, interaction.OverlayLines)
	}
}

func TestInteractionHelpEscapeAndQuitSemantics(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	interaction.Handle(Key{Kind: KeyRune, Rune: '?'})
	if !interaction.Help {
		t.Fatal("help not shown")
	}
	interaction.Handle(Key{Kind: KeyEscape})
	if interaction.Help {
		t.Fatal("help not dismissed")
	}
	if !interaction.Handle(Key{Kind: KeyRune, Rune: 'q'}).Quit {
		t.Fatal("q should quit outside command mode")
	}
}

func TestCardDensityShortcutsPersistAndClamp(t *testing.T) {
	interaction := NewInteraction(controllerFixture())
	if got := interaction.Controller.Config.Cards; got != 0 {
		t.Fatalf("initial cards = %d", got)
	}
	out := interaction.Handle(Key{Kind: KeyRune, Rune: ']'})
	if interaction.Controller.Config.Cards != 4 || !out.SaveConfig {
		t.Fatalf("] => cards=%d outcome=%#v", interaction.Controller.Config.Cards, out)
	}
	out = interaction.Handle(Key{Kind: KeyRune, Rune: '['})
	if interaction.Controller.Config.Cards != 3 || !out.SaveConfig {
		t.Fatalf("[ => cards=%d outcome=%#v", interaction.Controller.Config.Cards, out)
	}
	out = interaction.Handle(Key{Kind: KeyRune, Rune: '0'})
	if interaction.Controller.Config.Cards != 0 || !out.SaveConfig {
		t.Fatalf("0 => cards=%d outcome=%#v", interaction.Controller.Config.Cards, out)
	}
	interaction.Controller.Config.Cards = 8
	interaction.Handle(Key{Kind: KeyRune, Rune: ']'})
	if interaction.Controller.Config.Cards != 8 {
		t.Fatalf("cards should clamp at 8, got %d", interaction.Controller.Config.Cards)
	}
	interaction.Controller.Config.Cards = 1
	interaction.Handle(Key{Kind: KeyRune, Rune: '['})
	if interaction.Controller.Config.Cards != 1 {
		t.Fatalf("cards should clamp at 1, got %d", interaction.Controller.Config.Cards)
	}
}
