package ui

// Layout is the responsive geometry for one complete frame.
type Layout struct {
	Width          int
	Height         int
	HeaderHeight   int
	FooterHeight   int
	ContentHeight  int
	VisibleColumns int
	StartColumn    int
	ColumnWidth    int
	Gap            int
	CardHeight     int
	Tiny           bool
}

func ComputeLayout(width, height, totalColumns, active int, mode string) Layout {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	tiny := width < 56 || height < 18
	header := 3
	footer := 2
	if height < 12 {
		header = 2
		footer = 1
	}
	if header+footer >= height {
		header = 1
		footer = 0
	}
	content := height - header - footer
	if content < 1 {
		content = 1
	}
	visible := 1
	if mode != "stream" {
		switch {
		case width >= 132:
			visible = 3
		case width >= 86:
			visible = 2
		}
	}
	if totalColumns > 0 && visible > totalColumns {
		visible = totalColumns
	}
	if visible < 1 {
		visible = 1
	}
	if active < 0 {
		active = 0
	}
	if totalColumns > 0 && active >= totalColumns {
		active = totalColumns - 1
	}
	start := active - visible + 1
	if start < 0 {
		start = 0
	}
	if totalColumns > visible && start > totalColumns-visible {
		start = totalColumns - visible
	}
	gap := 1
	columnWidth := (width - gap*(visible-1)) / visible
	if columnWidth < 1 {
		columnWidth = 1
	}
	cardHeight := 12
	if columnWidth >= 70 {
		cardHeight = 13
	}
	if content < cardHeight+2 {
		cardHeight = content - 2
	}
	if cardHeight < 5 {
		cardHeight = content
	}
	if cardHeight < 1 {
		cardHeight = 1
	}
	return Layout{
		Width: width, Height: height, HeaderHeight: header, FooterHeight: footer,
		ContentHeight: content, VisibleColumns: visible, StartColumn: start,
		ColumnWidth: columnWidth, Gap: gap, CardHeight: cardHeight, Tiny: tiny,
	}
}
