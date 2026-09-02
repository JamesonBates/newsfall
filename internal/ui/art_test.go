package ui

import (
	"image"
	"image/color"
	"reflect"
	"testing"
)

func TestFallbackArtIsDeterministicColorfulAndSized(t *testing.T) {
	theme := ThemeByName("aurora")
	first := FallbackArt("same story", 14, 5, theme, true)
	second := FallbackArt("same story", 14, 5, theme, true)
	other := FallbackArt("another story", 14, 5, theme, true)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("fallback art should be deterministic")
	}
	if reflect.DeepEqual(first, other) {
		t.Fatal("different seeds should make different art")
	}
	if len(first) != 5 {
		t.Fatalf("height = %d", len(first))
	}
	for _, line := range first {
		if VisibleWidth(line) != 14 {
			t.Fatalf("line width = %d: %q", VisibleWidth(line), line)
		}
	}
}

func TestImageArtUsesHalfBlocksAtRequestedDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(3, 3, color.RGBA{B: 255, A: 255})
	got := ImageArt(img, 10, 3, ThemeByName("ocean"), true)
	if len(got) != 3 {
		t.Fatalf("height = %d", len(got))
	}
	for _, line := range got {
		if VisibleWidth(line) != 10 {
			t.Fatalf("line width = %d", VisibleWidth(line))
		}
	}
}
