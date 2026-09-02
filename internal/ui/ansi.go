package ui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const ansiReset = "\x1b[0m"

// FG applies a 24-bit foreground color.
func FG(hexColor, value string) string {
	r, g, b := parseHex(hexColor)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", r, g, b, value, ansiReset)
}

// BG applies a 24-bit background color.
func BG(hexColor, value string) string {
	r, g, b := parseHex(hexColor)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm%s%s", r, g, b, value, ansiReset)
}

func Bold(value string) string      { return "\x1b[1m" + value + ansiReset }
func Dim(value string) string       { return "\x1b[2m" + value + ansiReset }
func Underline(value string) string { return "\x1b[4m" + value + ansiReset }

// Hyperlink emits the widely supported OSC 8 terminal hyperlink sequence.
func Hyperlink(rawURL, label string) string {
	rawURL = safeControlFree(rawURL)
	if rawURL == "" {
		return label
	}
	return "\x1b]8;;" + rawURL + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

// StripANSI removes CSI, OSC, and short escape sequences.
func StripANSI(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != 0x1b {
			r, size := utf8.DecodeRuneInString(value[i:])
			out.WriteRune(r)
			i += size
			continue
		}
		i++
		if i >= len(value) {
			break
		}
		switch value[i] {
		case '[':
			i++
			for i < len(value) {
				b := value[i]
				i++
				if b >= 0x40 && b <= 0x7e {
					break
				}
			}
		case ']':
			i++
			for i < len(value) {
				if value[i] == 0x07 {
					i++
					break
				}
				if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			// Two-byte escape sequence.
			i++
		}
	}
	return out.String()
}

func VisibleWidth(value string) int {
	width := 0
	for _, r := range StripANSI(value) {
		width += runeWidth(r)
	}
	return width
}

// FitANSI truncates or right-pads a styled line to exactly width cells.
func FitANSI(value string, width int) string {
	if width <= 0 {
		return ""
	}
	current := VisibleWidth(value)
	if current == width {
		return value
	}
	if current < width {
		return value + strings.Repeat(" ", width-current)
	}
	return truncateANSI(value, width)
}

func truncateANSI(value string, width int) string {
	if width <= 0 {
		return ""
	}
	limit := width
	ellipsis := ""
	if width > 1 {
		limit = width - 1
		ellipsis = "…"
	}
	var out strings.Builder
	cells := 0
	for i := 0; i < len(value); {
		if value[i] == 0x1b {
			start := i
			i = escapeEnd(value, i)
			out.WriteString(value[start:i])
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		rw := runeWidth(r)
		if cells+rw > limit {
			break
		}
		out.WriteRune(r)
		cells += rw
		i += size
	}
	out.WriteString(ellipsis)
	if strings.Contains(value, "\x1b") {
		out.WriteString(ansiReset)
	}
	visible := cells + VisibleWidth(ellipsis)
	if visible < width {
		out.WriteString(strings.Repeat(" ", width-visible))
	}
	return out.String()
}

func escapeEnd(value string, start int) int {
	i := start + 1
	if i >= len(value) {
		return i
	}
	switch value[i] {
	case '[':
		i++
		for i < len(value) {
			b := value[i]
			i++
			if b >= 0x40 && b <= 0x7e {
				break
			}
		}
	case ']':
		i++
		for i < len(value) {
			if value[i] == 0x07 {
				return i + 1
			}
			if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
				return i + 2
			}
			i++
		}
	default:
		i++
	}
	return i
}

// WrapText greedily wraps plain text and marks omitted content with an ellipsis.
func WrapText(value string, width, maxLines int) []string {
	if width <= 0 || maxLines <= 0 {
		return nil
	}
	words := strings.Fields(safeControlFree(value))
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current string
	for _, word := range words {
		for VisibleWidth(word) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			part, rest := splitAtWidth(word, width)
			lines = append(lines, part)
			word = rest
		}
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if VisibleWidth(candidate) <= width {
			current = candidate
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]
	last := lines[maxLines-1]
	if width == 1 {
		last = "…"
	} else {
		last = truncatePlain(last, width-1) + "…"
	}
	lines[maxLines-1] = last
	return lines
}

func splitAtWidth(value string, width int) (string, string) {
	if width <= 0 {
		return "", value
	}
	cells := 0
	index := 0
	for i, r := range value {
		rw := runeWidth(r)
		if cells+rw > width {
			break
		}
		cells += rw
		index = i + utf8.RuneLen(r)
	}
	if index == 0 {
		_, size := utf8.DecodeRuneInString(value)
		index = size
	}
	return value[:index], value[index:]
}

func truncatePlain(value string, width int) string {
	part, _ := splitAtWidth(value, width)
	return strings.TrimRight(part, " ")
}

func safeControlFree(value string) string {
	var out strings.Builder
	for _, r := range value {
		if r == 0x1b || unicode.IsControl(r) {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func runeWidth(r rune) int {
	if r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == '\u200d' || (r >= 0xfe00 && r <= 0xfe0f) {
		return 0
	}
	if r < 32 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) ||
		(r >= 0x20000 && r <= 0x3fffd))
}
