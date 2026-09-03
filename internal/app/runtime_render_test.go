package app

import (
	"bytes"
	"image"
	"strings"
	"testing"
	"time"
)

func TestDrawUsesCRLFForRowsInRawTerminalMode(t *testing.T) {
	var output bytes.Buffer
	r := runner{
		runtimeModel: runtimeModel{
			interaction: NewInteraction(controllerFixture()),
			width:       100,
			height:      24,
			images:      map[string]image.Image{},
		},
		stdout: &output,
	}

	if err := r.draw(time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	frame := output.String()
	for index := strings.IndexByte(frame, '\n'); index >= 0; {
		if index == 0 || frame[index-1] != '\r' {
			t.Fatalf("draw emitted a bare line feed at byte %d", index)
		}
		next := strings.IndexByte(frame[index+1:], '\n')
		if next < 0 {
			break
		}
		index += next + 1
	}
}
