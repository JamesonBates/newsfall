package content

import "testing"

func TestCleanTextRemovesMarkupScriptsAndTerminalControls(t *testing.T) {
	input := `<p>Hello &amp; <strong>world</strong></p><script>alert(1)</script>` + "\x1b[31mRED\x1b[0m\x00"
	got := CleanText(input)
	want := "Hello & world RED"
	if got != want {
		t.Fatalf("CleanText = %q, want %q", got, want)
	}
}

func TestExcerptPrefersAWordBoundary(t *testing.T) {
	got := Excerpt("one two three four five", 14)
	if got != "one two three…" {
		t.Fatalf("Excerpt = %q", got)
	}
	if got := Excerpt("short", 14); got != "short" {
		t.Fatalf("short Excerpt = %q", got)
	}
}
