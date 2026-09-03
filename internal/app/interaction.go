package app

import (
	"fmt"
	"strings"
	"unicode"

	"newsfall/internal/command"
)

// Outcome describes side effects the terminal runtime performs after a key.
type Outcome struct {
	Quit       bool
	Refresh    bool
	SaveConfig bool
	Reload     bool
	OpenURL    string
}

// Interaction owns keyboard-facing state and delegates article navigation to
// Controller. It intentionally performs no filesystem, process, or network I/O.
type Interaction struct {
	Controller   *Controller
	Paused       bool
	Help         bool
	CommandMode  bool
	CommandText  string
	OverlayTitle string
	OverlayLines []string
	SourceErrors []string
	Status       string
}

func NewInteraction(controller *Controller) *Interaction {
	return &Interaction{Controller: controller, Status: "ready"}
}

func (i *Interaction) Handle(key Key) Outcome {
	if i.Controller == nil {
		return Outcome{}
	}
	if key.Kind == KeyCtrlC {
		return Outcome{Quit: true}
	}
	if i.CommandMode {
		return i.handleCommandKey(key)
	}
	if key.Kind == KeyEscape {
		i.Help = false
		i.OverlayTitle = ""
		i.OverlayLines = nil
		return Outcome{}
	}
	if len(i.OverlayLines) > 0 {
		i.OverlayTitle = ""
		i.OverlayLines = nil
	}

	switch key.Kind {
	case KeyUp:
		i.Controller.MoveVertical(-1)
	case KeyDown:
		i.Controller.MoveVertical(1)
	case KeyLeft:
		i.Controller.MoveHorizontal(-1)
	case KeyRight:
		i.Controller.MoveHorizontal(1)
	case KeyHome:
		i.Controller.First()
	case KeyEnd:
		i.Controller.Last()
	case KeyEnter:
		return i.openSelected()
	case KeyTab:
		i.Controller.CycleTheme()
		i.Status = "theme · " + i.Controller.Config.Theme
		return Outcome{SaveConfig: true}
	case KeyRune:
		return i.handleRune(key.Rune)
	}
	return Outcome{}
}

func (i *Interaction) handleRune(r rune) Outcome {
	switch r {
	case 'q':
		return Outcome{Quit: true}
	case '?':
		i.Help = !i.Help
		i.OverlayLines = nil
		return Outcome{}
	case ':':
		i.Help = false
		i.OverlayLines = nil
		i.CommandMode = true
		i.CommandText = ""
		i.Status = "enter a command · :help lists syntax"
		return Outcome{}
	case 'j':
		i.Controller.MoveVertical(1)
	case 'k':
		i.Controller.MoveVertical(-1)
	case 'h':
		i.Controller.MoveHorizontal(-1)
	case 'l':
		i.Controller.MoveHorizontal(1)
	case 'g':
		i.Controller.First()
	case 'G':
		i.Controller.Last()
	case 'o':
		return i.openSelected()
	case 'r':
		i.Status = "refresh requested"
		return Outcome{Refresh: true}
	case 'e':
		i.showSourceErrors()
	case 'p':
		i.Paused = !i.Paused
		if i.Paused {
			i.Status = "refresh and ambient drift paused"
		} else {
			i.Status = "live refresh resumed"
		}
	case 'a':
		i.Controller.Config.Ambient = !i.Controller.Config.Ambient
		i.Status = "ambient drift " + boolLabel(i.Controller.Config.Ambient)
		return Outcome{SaveConfig: true}
	case 'm':
		if i.Controller.Config.Mode == "stream" {
			i.Controller.Config.Mode = "deck"
		} else {
			i.Controller.Config.Mode = "stream"
		}
		i.Status = "mode · " + i.Controller.Config.Mode
		return Outcome{SaveConfig: true}
	case 'i':
		i.Controller.Config.Images = !i.Controller.Config.Images
		i.Status = "article images " + boolLabel(i.Controller.Config.Images)
		return Outcome{SaveConfig: true}
	}
	return Outcome{}
}

func (i *Interaction) handleCommandKey(key Key) Outcome {
	switch key.Kind {
	case KeyEscape:
		i.CommandMode = false
		i.CommandText = ""
		i.Status = "command cancelled"
		return Outcome{}
	case KeyBackspace, KeyDelete:
		runes := []rune(i.CommandText)
		if len(runes) > 0 {
			i.CommandText = string(runes[:len(runes)-1])
		}
		return Outcome{}
	case KeyEnter:
		line := strings.TrimSpace(i.CommandText)
		i.CommandMode = false
		i.CommandText = ""
		if line == "" {
			i.Status = "command cancelled"
			return Outcome{}
		}
		parsed, parseErr := command.Parse(line)
		if parseErr == nil && parsed.Name == "feed" && len(parsed.Args) == 1 && strings.EqualFold(parsed.Args[0], "errors") {
			i.showSourceErrors()
			return Outcome{}
		}
		next, effect, err := command.Execute(i.Controller.Config, line)
		if err != nil {
			i.Status = "command failed · " + err.Error()
			i.OverlayTitle = "COMMAND ERROR"
			i.OverlayLines = []string{err.Error(), "", "Press Esc to return, then use :help for command syntax."}
			return Outcome{}
		}
		if effect.Save {
			i.Controller.SetConfig(next)
		}
		i.Status = effect.Message
		if parseErr == nil && (parsed.Name == "help" || parsed.Name == "?") {
			i.Help = true
		} else if len(effect.Output) > 0 {
			i.OverlayTitle = commandTitle(parsed)
			i.OverlayLines = append([]string(nil), effect.Output...)
		}
		return Outcome{SaveConfig: effect.Save, Refresh: effect.Refresh, Reload: effect.Reload}
	case KeyRune:
		if unicode.IsPrint(key.Rune) {
			i.CommandText += string(key.Rune)
		}
	}
	return Outcome{}
}

func (i *Interaction) showSourceErrors() {
	if len(i.SourceErrors) == 0 {
		i.OverlayTitle = ""
		i.OverlayLines = nil
		i.Status = "no source errors from the latest refresh"
		return
	}
	i.Help = false
	i.OverlayTitle = "SOURCE ERRORS"
	i.OverlayLines = append([]string(nil), i.SourceErrors...)
	i.Status = fmt.Sprintf("%d source %s · press Esc to return", len(i.SourceErrors), plural(len(i.SourceErrors), "error", "errors"))
}

func (i *Interaction) openSelected() Outcome {
	article, ok := i.Controller.SelectedArticle()
	if !ok || strings.TrimSpace(article.URL) == "" {
		i.Status = "selected story has no article link"
		return Outcome{}
	}
	delete(i.Controller.NewIDs, article.ID)
	i.Status = "opening · " + article.Source
	return Outcome{OpenURL: article.URL}
}

func commandTitle(parsed command.Command) string {
	title := strings.ToUpper(parsed.Name)
	if len(parsed.Args) > 0 {
		title += " " + strings.ToUpper(parsed.Args[0])
	}
	return title
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
