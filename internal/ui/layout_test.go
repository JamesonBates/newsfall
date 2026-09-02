package ui

import "testing"

func TestComputeLayoutResponsiveBreakpointsAndActiveVisibility(t *testing.T) {
	tests := []struct {
		width, height, total, active int
		mode                         string
		visible                      int
	}{
		{180, 48, 4, 0, "deck", 3},
		{105, 38, 4, 3, "deck", 2},
		{72, 32, 4, 2, "deck", 1},
		{180, 48, 4, 2, "stream", 1},
	}
	for _, test := range tests {
		got := ComputeLayout(test.width, test.height, test.total, test.active, test.mode)
		if got.VisibleColumns != test.visible {
			t.Errorf("%dx%d %s visible = %d, want %d", test.width, test.height, test.mode, got.VisibleColumns, test.visible)
		}
		if got.ColumnWidth <= 0 || got.ContentHeight <= 0 || got.CardHeight <= 0 {
			t.Errorf("invalid dimensions: %#v", got)
		}
		if test.mode == "deck" && test.total > 0 && (test.active < got.StartColumn || test.active >= got.StartColumn+got.VisibleColumns) {
			t.Errorf("active column not visible: %#v", got)
		}
	}
}

func TestComputeLayoutTinyTerminalNeverReturnsNegativeGeometry(t *testing.T) {
	got := ComputeLayout(34, 10, 3, 2, "deck")
	if !got.Tiny {
		t.Fatal("expected tiny layout")
	}
	if got.ContentHeight < 1 || got.ColumnWidth < 1 || got.HeaderHeight < 1 || got.FooterHeight < 0 {
		t.Fatalf("layout = %#v", got)
	}
}
