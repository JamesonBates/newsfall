package model

import (
	"testing"
	"time"
)

func TestStableIDPreferenceAndDeterminism(t *testing.T) {
	when := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	if got := StableID("guid-1", "https://example.com/a", "A", when); got != "guid:guid-1" {
		t.Fatalf("GUID ID = %q", got)
	}
	urlID := StableID("", "HTTPS://Example.com/a/?utm_source=x#frag", "A", when)
	if urlID != StableID("", "https://example.com/a", "Different", when.Add(time.Hour)) {
		t.Fatalf("canonical URL should determine ID: %q", urlID)
	}
	a := StableID("", "", "A title", when)
	b := StableID("", "", "A title", when)
	if a != b || a == "" {
		t.Fatalf("hash IDs not deterministic: %q %q", a, b)
	}
}

func TestCanonicalURLRemovesOnlyTrackingNoise(t *testing.T) {
	got := CanonicalURL("https://EXAMPLE.com/story/?utm_medium=rss&keep=1&fbclid=x#comments")
	want := "https://example.com/story?keep=1"
	if got != want {
		t.Fatalf("CanonicalURL = %q, want %q", got, want)
	}
}
