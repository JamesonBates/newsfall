package platform

import (
	"reflect"
	"testing"
)

func TestOpenCommandIsArgumentSafeAcrossSupportedPlatforms(t *testing.T) {
	url := "https://example.com/story?a=1;touch=/tmp/nope"
	tests := []struct {
		goos string
		name string
		args []string
	}{
		{"darwin", "open", []string{url}},
		{"linux", "xdg-open", []string{url}},
	}
	for _, test := range tests {
		name, args, err := OpenCommand(test.goos, url)
		if err != nil {
			t.Fatalf("%s: %v", test.goos, err)
		}
		if name != test.name || !reflect.DeepEqual(args, test.args) {
			t.Fatalf("%s = %q %#v", test.goos, name, args)
		}
	}
}

func TestOpenCommandRejectsUnsafeOrUnsupportedURLs(t *testing.T) {
	for _, input := range []string{"javascript:alert(1)", "file:///etc/passwd", "https://example.com/\x1b[2J"} {
		if _, _, err := OpenCommand("darwin", input); err == nil {
			t.Fatalf("%q expected error", input)
		}
	}
	if _, _, err := OpenCommand("plan9", "https://example.com"); err == nil {
		t.Fatal("unsupported platform should fail")
	}
}
