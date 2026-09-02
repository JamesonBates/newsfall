package ui

import (
	"strings"
	"testing"
)

func TestVisibleWidthFitAndStripANSI(t *testing.T) {
	styled := FG("#22D3EE", "SIGNAL") + " " + Bold("LIVE")
	if got := VisibleWidth(styled); got != 11 {
		t.Fatalf("width = %d", got)
	}
	if strings.Contains(StripANSI(styled), "\x1b") || StripANSI(styled) != "SIGNAL LIVE" {
		t.Fatalf("strip = %q", StripANSI(styled))
	}
	fitted := FitANSI(styled, 8)
	if VisibleWidth(fitted) != 8 {
		t.Fatalf("fitted width = %d: %q", VisibleWidth(fitted), fitted)
	}
}

func TestWrapTextRespectsRuneWidthAndLineLimit(t *testing.T) {
	got := WrapText("Alpha beta gamma delta epsilon", 11, 2)
	if len(got) != 2 || VisibleWidth(got[0]) > 11 || VisibleWidth(got[1]) > 11 || !strings.HasSuffix(got[1], "…") {
		t.Fatalf("wrapped = %#v", got)
	}
}
