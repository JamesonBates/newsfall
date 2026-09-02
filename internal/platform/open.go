package platform

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

func OpenCommand(goos, rawURL string) (string, []string, error) {
	if strings.IndexFunc(rawURL, func(r rune) bool { return r == 0x1b || unicode.IsControl(r) }) >= 0 {
		return "", nil, errors.New("URL contains control characters")
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", nil, fmt.Errorf("unsupported article URL %q", rawURL)
	}
	switch goos {
	case "darwin":
		return "open", []string{parsed.String()}, nil
	case "linux":
		return "xdg-open", []string{parsed.String()}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", parsed.String()}, nil
	default:
		return "", nil, fmt.Errorf("opening a browser is unsupported on %s", goos)
	}
}

func OpenURL(rawURL string) error {
	name, args, err := OpenCommand(runtime.GOOS, rawURL)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start()
}
