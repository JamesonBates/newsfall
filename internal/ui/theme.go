package ui

import (
	"strconv"
	"strings"
)

// Theme defines the small palette shared by the dashboard and generated art.
type Theme struct {
	Name       string
	Background string
	Panel      string
	PanelAlt   string
	Text       string
	Muted      string
	Faint      string
	Accent     string
	Accent2    string
	Warning    string
	Success    string
	Palette    []string
}

func ThemeByName(name string) Theme {
	switch strings.ToLower(name) {
	case "ember":
		return Theme{Name: "ember", Background: "#130B08", Panel: "#24120D", PanelAlt: "#321910", Text: "#FFF2E8", Muted: "#B99781", Faint: "#6B4A3B", Accent: "#FF6B35", Accent2: "#FFB703", Warning: "#FF4D6D", Success: "#84CC16", Palette: []string{"#FF6B35", "#FFB703", "#F72585", "#FB8500", "#8338EC"}}
	case "ocean":
		return Theme{Name: "ocean", Background: "#04121C", Panel: "#072638", PanelAlt: "#0A3349", Text: "#E5FAFF", Muted: "#74A9BD", Faint: "#315A6B", Accent: "#00D4FF", Accent2: "#38BDF8", Warning: "#F59E0B", Success: "#2DD4BF", Palette: []string{"#00D4FF", "#38BDF8", "#2DD4BF", "#6366F1", "#06B6D4"}}
	case "mono":
		return Theme{Name: "mono", Background: "#080808", Panel: "#161616", PanelAlt: "#242424", Text: "#F5F5F5", Muted: "#A3A3A3", Faint: "#525252", Accent: "#FFFFFF", Accent2: "#D4D4D4", Warning: "#E5E5E5", Success: "#FAFAFA", Palette: []string{"#F5F5F5", "#D4D4D4", "#A3A3A3", "#737373", "#404040"}}
	default:
		return Theme{Name: "aurora", Background: "#050A12", Panel: "#0A1624", PanelAlt: "#0E2033", Text: "#EAF4FF", Muted: "#7890A8", Faint: "#32485D", Accent: "#8B5CF6", Accent2: "#22D3EE", Warning: "#FB7185", Success: "#34D399", Palette: []string{"#8B5CF6", "#22D3EE", "#F472B6", "#34D399", "#F59E0B"}}
	}
}

func parseHex(value string) (int, int, int) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		value = string([]byte{value[0], value[0], value[1], value[1], value[2], value[2]})
	}
	if len(value) != 6 {
		return 255, 255, 255
	}
	parsed, err := strconv.ParseUint(value, 16, 24)
	if err != nil {
		return 255, 255, 255
	}
	return int(parsed >> 16), int((parsed >> 8) & 0xff), int(parsed & 0xff)
}
