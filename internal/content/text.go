package content

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	scriptBlock = regexp.MustCompile(`(?is)<script[^>]*>.*?</script\s*>`)
	styleBlock  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style\s*>`)
	ansiCSI     = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC     = regexp.MustCompile(`\x1b\][^\x07]*(?:\x07|\x1b\\)`)
)

// CleanText converts hostile or messy feed HTML into terminal-safe plain text.
func CleanText(value string) string {
	value = scriptBlock.ReplaceAllString(value, " ")
	value = styleBlock.ReplaceAllString(value, " ")
	value = ansiOSC.ReplaceAllString(value, "")
	value = ansiCSI.ReplaceAllString(value, "")
	value = stripTags(value)
	value = html.UnescapeString(value)
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsControl(r) {
			if r == '\n' || r == '\r' || r == '\t' {
				b.WriteByte(' ')
			}
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func stripTags(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	inside := false
	for _, r := range value {
		switch r {
		case '<':
			inside = true
			b.WriteByte(' ')
		case '>':
			inside = false
			b.WriteByte(' ')
		default:
			if !inside {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// Excerpt limits text by rune count and avoids cutting in the middle of a word
// when a useful boundary is available.
func Excerpt(value string, maxRunes int) string {
	value = CleanText(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	limit := maxRunes - 1
	if limit <= 0 {
		return "…"
	}
	runes := []rune(value)
	cut := limit
	for cut > limit/2 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	if cut <= limit/2 {
		cut = limit
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}
