package term

import (
	"strings"
	"testing"
)

func TestParseSizeAcceptsSttyRowsColumns(t *testing.T) {
	width, height, err := ParseSize("48 160\n")
	if err != nil || width != 160 || height != 48 {
		t.Fatalf("size = %dx%d, %v", width, height, err)
	}
	for _, input := range []string{"", "wide tall", "0 80", "24"} {
		if _, _, err := ParseSize(input); err == nil {
			t.Fatalf("%q expected error", input)
		}
	}
}

func TestScreenSequencesManageAlternateScreenCursorAndPaste(t *testing.T) {
	if !strings.Contains(EnterSequence, "?1049h") || !strings.Contains(EnterSequence, "?25l") || !strings.Contains(EnterSequence, "?2004h") || !strings.Contains(EnterSequence, "?7l") {
		t.Fatalf("enter sequence = %q", EnterSequence)
	}
	if !strings.Contains(ExitSequence, "?1049l") || !strings.Contains(ExitSequence, "?25h") || !strings.Contains(ExitSequence, "?2004l") || !strings.Contains(ExitSequence, "?7h") {
		t.Fatalf("exit sequence = %q", ExitSequence)
	}
}
