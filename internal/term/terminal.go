// Package term owns Newsfall's small, dependency-free terminal boundary.
package term

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	EnterSequence = "\x1b[?1049h\x1b[?25l\x1b[?7l\x1b[?2004h\x1b[2J\x1b[H"
	ExitSequence  = "\x1b[?2004l\x1b[?7h\x1b[?25h\x1b[0m\x1b[?1049l"
	HomeSequence  = "\x1b[H"
)

// Session restores both terminal flags and screen state exactly once.
type Session struct {
	stdin    *os.File
	stdout   io.Writer
	original []string
	once     sync.Once
	err      error
}

func Begin(stdin *os.File, stdout io.Writer) (*Session, error) {
	if stdin == nil || stdout == nil {
		return nil, errors.New("terminal input and output are required")
	}
	originalRaw, err := runStty(stdin, "-g")
	if err != nil {
		return nil, fmt.Errorf("Newsfall needs an interactive terminal: %w", err)
	}
	original := strings.Fields(strings.TrimSpace(originalRaw))
	if len(original) == 0 {
		return nil, errors.New("stty returned no restorable terminal state")
	}
	if _, err := runStty(stdin, "raw", "-echo", "min", "0", "time", "1"); err != nil {
		return nil, fmt.Errorf("enable raw terminal mode: %w", err)
	}
	session := &Session{stdin: stdin, stdout: stdout, original: original}
	if _, err := io.WriteString(stdout, EnterSequence); err != nil {
		_, _ = runStty(stdin, original...)
		return nil, fmt.Errorf("enter terminal screen: %w", err)
	}
	return session, nil
}

func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		var errs []error
		if _, err := io.WriteString(s.stdout, ExitSequence); err != nil {
			errs = append(errs, err)
		}
		if _, err := runStty(s.stdin, s.original...); err != nil {
			errs = append(errs, err)
		}
		s.err = errors.Join(errs...)
	})
	return s.err
}

func CurrentSize(stdin *os.File) (width, height int) {
	if stdin != nil {
		if output, err := runStty(stdin, "size"); err == nil {
			if width, height, err = ParseSize(output); err == nil {
				return width, height
			}
		}
	}
	width, _ = strconv.Atoi(os.Getenv("COLUMNS"))
	height, _ = strconv.Atoi(os.Getenv("LINES"))
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 38
	}
	return width, height
}

// ParseSize converts the portable `stty size` rows/columns format.
func ParseSize(value string) (width, height int, err error) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected terminal size %q", strings.TrimSpace(value))
	}
	height, err = strconv.Atoi(fields[0])
	if err != nil || height <= 0 {
		return 0, 0, fmt.Errorf("invalid terminal rows %q", fields[0])
	}
	width, err = strconv.Atoi(fields[1])
	if err != nil || width <= 0 {
		return 0, 0, fmt.Errorf("invalid terminal columns %q", fields[1])
	}
	return width, height, nil
}

func runStty(stdin *os.File, args ...string) (string, error) {
	command := exec.Command("stty", args...)
	command.Stdin = stdin
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return string(output), nil
}
