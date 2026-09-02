package ui

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strings"
)

// FallbackArt creates deterministic cover art from an article seed.
func FallbackArt(seed string, width, height int, theme Theme, colorEnabled bool) []string {
	if width < 1 || height < 1 {
		return nil
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	state := h.Sum64()
	if state == 0 {
		state = 0x9e3779b97f4a7c15
	}
	palette := theme.Palette
	if len(palette) == 0 {
		palette = []string{theme.Accent, theme.Accent2, theme.Text}
	}
	chars := []rune(" ·•:+x◆◇╱╲░▒▓")
	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var line strings.Builder
		for x := 0; x < width; x++ {
			state = xorshift(state + uint64((x+1)*(y+3)))
			wave := math.Sin(float64(x+y*2) + float64(state&31)/7.0)
			index := int((state >> 8) % uint64(len(palette)))
			other := (index + 1 + int(state%uint64(maxInt(1, len(palette)-1)))) % len(palette)
			if colorEnabled {
				top := palette[index]
				bottom := palette[other]
				if wave < -0.15 {
					top, bottom = theme.PanelAlt, palette[index]
				}
				line.WriteString(halfBlock(top, bottom))
			} else {
				charIndex := int((state + uint64(x*y+1)) % uint64(len(chars)))
				line.WriteRune(chars[charIndex])
			}
		}
		if colorEnabled {
			line.WriteString(ansiReset)
		}
		lines[y] = line.String()
	}
	return lines
}

// ImageArt samples two source rows for each terminal row and emits true-color
// half blocks. In plain mode, luminance is mapped to a compact character ramp.
func ImageArt(img image.Image, width, height int, theme Theme, colorEnabled bool) []string {
	if img == nil || width < 1 || height < 1 {
		return nil
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return FallbackArt("empty-image", width, height, theme, colorEnabled)
	}
	background := hexColor(theme.Panel)
	lines := make([]string, height)
	ramp := []rune(" .:-=+*#%@")
	for y := 0; y < height; y++ {
		var line strings.Builder
		for x := 0; x < width; x++ {
			top := sampleCover(img, x, y*2, width, height*2, background)
			bottom := sampleCover(img, x, y*2+1, width, height*2, background)
			if colorEnabled {
				line.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B))
			} else {
				lum := (int(top.R)*299 + int(top.G)*587 + int(top.B)*114 + int(bottom.R)*299 + int(bottom.G)*587 + int(bottom.B)*114) / 2000
				index := lum * (len(ramp) - 1) / 255
				if index < 0 {
					index = 0
				}
				if index >= len(ramp) {
					index = len(ramp) - 1
				}
				line.WriteRune(ramp[index])
			}
		}
		if colorEnabled {
			line.WriteString(ansiReset)
		}
		lines[y] = line.String()
	}
	return lines
}

func sampleCover(img image.Image, x, y, targetW, targetH int, background color.RGBA) color.RGBA {
	bounds := img.Bounds()
	sourceAspect := float64(bounds.Dx()) / float64(bounds.Dy())
	targetAspect := float64(targetW) / float64(targetH)
	var sx, sy float64
	if sourceAspect > targetAspect {
		visibleW := float64(bounds.Dy()) * targetAspect
		offsetX := (float64(bounds.Dx()) - visibleW) / 2
		sx = offsetX + (float64(x)+0.5)*visibleW/float64(targetW)
		sy = (float64(y) + 0.5) * float64(bounds.Dy()) / float64(targetH)
	} else {
		visibleH := float64(bounds.Dx()) / targetAspect
		offsetY := (float64(bounds.Dy()) - visibleH) / 2
		sx = (float64(x) + 0.5) * float64(bounds.Dx()) / float64(targetW)
		sy = offsetY + (float64(y)+0.5)*visibleH/float64(targetH)
	}
	ix := bounds.Min.X + clampInt(int(sx), 0, bounds.Dx()-1)
	iy := bounds.Min.Y + clampInt(int(sy), 0, bounds.Dy()-1)
	r, g, b, a := img.At(ix, iy).RGBA()
	alpha := float64(a) / 65535
	return color.RGBA{
		R: uint8(float64(r>>8)*alpha + float64(background.R)*(1-alpha)),
		G: uint8(float64(g>>8)*alpha + float64(background.G)*(1-alpha)),
		B: uint8(float64(b>>8)*alpha + float64(background.B)*(1-alpha)),
		A: 255,
	}
}

func halfBlock(topHex, bottomHex string) string {
	tr, tg, tb := parseHex(topHex)
	br, bg, bb := parseHex(bottomHex)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb)
}

func hexColor(value string) color.RGBA {
	r, g, b := parseHex(value)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func xorshift(value uint64) uint64 {
	value ^= value << 13
	value ^= value >> 7
	value ^= value << 17
	return value
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
