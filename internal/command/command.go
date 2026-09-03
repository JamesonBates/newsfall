// Package command parses and applies Newsfall's in-app configuration commands.
package command

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"newsfall/internal/config"
)

// Command is a shell-like command line split into a verb and arguments.
type Command struct {
	Name string
	Args []string
}

// Effect tells the runtime what work should follow a successful command.
type Effect struct {
	Save    bool
	Refresh bool
	Reload  bool
	Message string
	Output  []string
}

// Parse accepts command lines both with and without Newsfall's leading colon.
func Parse(line string) (Command, error) {
	line = strings.TrimSpace(line)
	line = strings.TrimSpace(strings.TrimPrefix(line, ":"))
	if line == "" {
		return Command{}, errors.New("empty command; try :help")
	}
	words, err := splitWords(line)
	if err != nil {
		return Command{}, err
	}
	if len(words) == 0 {
		return Command{}, errors.New("empty command; try :help")
	}
	return Command{Name: strings.ToLower(words[0]), Args: words[1:]}, nil
}

// Execute parses and applies a command to a cloned configuration.
func Execute(cfg config.Config, line string) (config.Config, Effect, error) {
	cmd, err := Parse(line)
	if err != nil {
		return cfg, Effect{}, err
	}
	return Apply(cfg, cmd)
}

// Apply mutates a deep copy of cfg and validates the result before returning it.
func Apply(cfg config.Config, cmd Command) (config.Config, Effect, error) {
	next := cloneConfig(cfg)
	var effect Effect
	var err error

	switch strings.ToLower(cmd.Name) {
	case "feed", "source":
		next, effect, err = applyFeed(next, cmd.Args)
	case "column", "col":
		next, effect, err = applyColumn(next, cmd.Args)
	case "topic":
		next, effect, err = applyTopic(next, cmd.Args)
	case "refresh":
		next, effect, err = applyDuration(next, "refresh", cmd.Args)
	case "drift":
		next, effect, err = applyDuration(next, "drift", cmd.Args)
	case "theme":
		next, effect, err = applySingleSetting(next, "theme", cmd.Args)
	case "mode":
		next, effect, err = applySingleSetting(next, "mode", cmd.Args)
	case "images":
		next, effect, err = applyToggle(next, "images", cmd.Args)
	case "ambient":
		next, effect, err = applyToggle(next, "ambient", cmd.Args)
	case "cards":
		next, effect, err = applyCards(next, cmd.Args)
	case "reload":
		if len(cmd.Args) != 0 {
			err = errors.New("usage: reload")
		} else {
			effect = Effect{Reload: true, Message: "configuration reloaded"}
		}
	case "help", "?":
		if len(cmd.Args) != 0 {
			err = errors.New("usage: help")
		} else {
			effect.Output = HelpLines()
			effect.Message = "command reference"
		}
	default:
		err = fmt.Errorf("unknown command %q; try :help", cmd.Name)
	}
	if err != nil {
		return cfg, Effect{}, err
	}
	if effect.Save {
		if err := config.Validate(&next); err != nil {
			return cfg, Effect{}, err
		}
	}
	return next, effect, nil
}

func applyFeed(cfg config.Config, args []string) (config.Config, Effect, error) {
	if len(args) == 0 {
		return cfg, Effect{}, errors.New("usage: feed add|remove|list|errors …")
	}
	switch strings.ToLower(args[0]) {
	case "add":
		if len(args) < 2 {
			return cfg, Effect{}, errors.New("usage: feed add <site-or-feed-url> [name…] [column]")
		}
		u, err := url.Parse(strings.TrimSpace(args[1]))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return cfg, Effect{}, fmt.Errorf("invalid feed URL %q", args[1])
		}
		definition := config.Feed{URL: u.String()}
		nameParts := append([]string(nil), args[2:]...)
		if len(nameParts) > 0 {
			last := nameParts[len(nameParts)-1]
			columns, matched, matchErr := matchColumnToken(cfg.Columns, last)
			if matchErr != nil {
				return cfg, Effect{}, matchErr
			}
			if matched {
				definition.Columns = columns
				nameParts = nameParts[:len(nameParts)-1]
			}
		}
		if len(nameParts) > 0 {
			definition.Name = strings.TrimSpace(strings.Join(nameParts, " "))
		}
		cfg.Feeds = append(cfg.Feeds, definition)
		if err := config.Validate(&cfg); err != nil {
			return cfg, Effect{}, err
		}
		name := cfg.Feeds[len(cfg.Feeds)-1].Name
		return cfg, Effect{Save: true, Refresh: true, Message: "added source " + name + " · checking now"}, nil
	case "remove", "rm", "delete":
		if len(args) != 2 {
			return cfg, Effect{}, errors.New("usage: feed remove <name-or-url>")
		}
		needle := strings.TrimSpace(args[1])
		index := findFeed(cfg.Feeds, needle)
		if index < 0 {
			return cfg, Effect{}, fmt.Errorf("feed %q not found", needle)
		}
		name := cfg.Feeds[index].Name
		cfg.Feeds = append(cfg.Feeds[:index], cfg.Feeds[index+1:]...)
		return cfg, Effect{Save: true, Refresh: true, Message: "removed feed " + name}, nil
	case "list", "ls":
		if len(args) != 1 {
			return cfg, Effect{}, errors.New("usage: feed list")
		}
		lines := make([]string, 0, len(cfg.Feeds))
		for _, feed := range cfg.Feeds {
			suffix := ""
			if len(feed.Columns) > 0 {
				suffix = "  → " + strings.Join(feed.Columns, ",")
			}
			lines = append(lines, fmt.Sprintf("%-20s %s%s", feed.Name, feed.URL, suffix))
		}
		return cfg, Effect{Output: lines, Message: fmt.Sprintf("%d feeds", len(lines))}, nil
	case "errors":
		if len(args) != 1 {
			return cfg, Effect{}, errors.New("usage: feed errors")
		}
		return cfg, Effect{Message: "source errors are available in the live app · press e"}, nil
	default:
		return cfg, Effect{}, fmt.Errorf("unknown feed action %q; use add, remove, list, or errors", args[0])
	}
}

func applyColumn(cfg config.Config, args []string) (config.Config, Effect, error) {
	if len(args) == 0 {
		return cfg, Effect{}, errors.New("usage: column add|remove|list …")
	}
	switch strings.ToLower(args[0]) {
	case "add":
		if len(args) < 3 {
			return cfg, Effect{}, errors.New("usage: column add <id> <title> [topics…]")
		}
		column := config.Column{ID: args[1], Title: args[2], Include: append([]string(nil), args[3:]...)}
		cfg.Columns = append(cfg.Columns, column)
		if err := config.Validate(&cfg); err != nil {
			return cfg, Effect{}, err
		}
		created := cfg.Columns[len(cfg.Columns)-1]
		return cfg, Effect{Save: true, Message: "added column " + created.Title}, nil
	case "remove", "rm", "delete":
		if len(args) != 2 {
			return cfg, Effect{}, errors.New("usage: column remove <id>")
		}
		index := findColumn(cfg.Columns, args[1])
		if index < 0 {
			return cfg, Effect{}, fmt.Errorf("column %q not found", args[1])
		}
		id, title := cfg.Columns[index].ID, cfg.Columns[index].Title
		cfg.Columns = append(cfg.Columns[:index], cfg.Columns[index+1:]...)
		for i := range cfg.Feeds {
			cfg.Feeds[i].Columns = removeFold(cfg.Feeds[i].Columns, id)
		}
		return cfg, Effect{Save: true, Message: "removed column " + title}, nil
	case "list", "ls":
		if len(args) != 1 {
			return cfg, Effect{}, errors.New("usage: column list")
		}
		lines := make([]string, 0, len(cfg.Columns))
		for _, column := range cfg.Columns {
			topics := "all assigned feeds"
			if len(column.Include) > 0 {
				topics = strings.Join(column.Include, ", ")
			}
			lines = append(lines, fmt.Sprintf("%-14s %-22s %s", column.ID, column.Title, topics))
		}
		return cfg, Effect{Output: lines, Message: fmt.Sprintf("%d columns", len(lines))}, nil
	default:
		return cfg, Effect{}, fmt.Errorf("unknown column action %q; use add, remove, or list", args[0])
	}
}

func applyTopic(cfg config.Config, args []string) (config.Config, Effect, error) {
	if len(args) < 3 {
		return cfg, Effect{}, errors.New("usage: topic add|remove <column> <term…>")
	}
	index := findColumn(cfg.Columns, args[1])
	if index < 0 {
		return cfg, Effect{}, fmt.Errorf("column %q not found", args[1])
	}
	terms := args[2:]
	switch strings.ToLower(args[0]) {
	case "add":
		cfg.Columns[index].Include = append(cfg.Columns[index].Include, terms...)
		if err := config.Validate(&cfg); err != nil {
			return cfg, Effect{}, err
		}
		return cfg, Effect{Save: true, Message: "updated topics for " + cfg.Columns[index].Title}, nil
	case "remove", "rm", "delete":
		for _, term := range terms {
			cfg.Columns[index].Include = removeFold(cfg.Columns[index].Include, term)
		}
		return cfg, Effect{Save: true, Message: "updated topics for " + cfg.Columns[index].Title}, nil
	default:
		return cfg, Effect{}, fmt.Errorf("unknown topic action %q; use add or remove", args[0])
	}
}

func applyDuration(cfg config.Config, name string, args []string) (config.Config, Effect, error) {
	if len(args) != 1 {
		return cfg, Effect{}, fmt.Errorf("usage: %s <duration>", name)
	}
	if name == "refresh" && strings.EqualFold(args[0], "now") {
		return cfg, Effect{Refresh: true, Message: "refreshing now"}, nil
	}
	duration, err := time.ParseDuration(args[0])
	if err != nil {
		return cfg, Effect{}, fmt.Errorf("invalid %s duration %q", name, args[0])
	}
	if name == "refresh" {
		cfg.Refresh = duration.String()
	} else {
		cfg.Drift = duration.String()
	}
	if err := config.Validate(&cfg); err != nil {
		return cfg, Effect{}, err
	}
	return cfg, Effect{Save: true, Message: fmt.Sprintf("%s set to %s", name, duration)}, nil
}

func applySingleSetting(cfg config.Config, name string, args []string) (config.Config, Effect, error) {
	if len(args) != 1 {
		return cfg, Effect{}, fmt.Errorf("usage: %s <value>", name)
	}
	value := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "theme" {
		cfg.Theme = value
	} else {
		cfg.Mode = value
	}
	if err := config.Validate(&cfg); err != nil {
		return cfg, Effect{}, err
	}
	return cfg, Effect{Save: true, Message: name + " set to " + value}, nil
}

func applyCards(cfg config.Config, args []string) (config.Config, Effect, error) {
	if len(args) != 1 {
		return cfg, Effect{}, errors.New("usage: cards auto|1-8")
	}
	value := strings.ToLower(strings.TrimSpace(args[0]))
	if value == "auto" {
		cfg.Cards = 0
		return cfg, Effect{Save: true, Message: "cards set to auto"}, nil
	}
	cards, err := strconv.Atoi(value)
	if err != nil || cards < 1 || cards > 8 {
		return cfg, Effect{}, fmt.Errorf("cards must be auto or a number from 1 to 8; got %q", args[0])
	}
	cfg.Cards = cards
	return cfg, Effect{Save: true, Message: fmt.Sprintf("cards set to %d", cards)}, nil
}

func applyToggle(cfg config.Config, name string, args []string) (config.Config, Effect, error) {
	if len(args) != 1 {
		return cfg, Effect{}, fmt.Errorf("usage: %s on|off", name)
	}
	value, err := parseBool(args[0])
	if err != nil {
		return cfg, Effect{}, fmt.Errorf("%s: %w", name, err)
	}
	if name == "images" {
		cfg.Images = value
	} else {
		cfg.Ambient = value
	}
	return cfg, Effect{Save: true, Message: fmt.Sprintf("%s %s", name, onOff(value))}, nil
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "yes", "1":
		return true, nil
	case "off", "false", "no", "0":
		return false, nil
	default:
		return false, errors.New("expected on or off")
	}
}

func splitWords(input string) ([]string, error) {
	var words []string
	var current strings.Builder
	var quote rune
	escaped := false
	started := false
	flush := func() {
		if started {
			words = append(words, current.String())
			current.Reset()
			started = false
		}
	}
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		current.WriteRune(r)
		started = true
	}
	if escaped {
		return nil, errors.New("unfinished escape at end of command")
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote in command")
	}
	flush()
	return words, nil
}

func findFeed(feeds []config.Feed, needle string) int {
	needle = strings.TrimSpace(needle)
	for i, feed := range feeds {
		if strings.EqualFold(feed.Name, needle) || strings.EqualFold(feed.URL, needle) {
			return i
		}
	}
	return -1
}

func findColumn(columns []config.Column, needle string) int {
	for i, column := range columns {
		if strings.EqualFold(column.ID, strings.TrimSpace(needle)) || strings.EqualFold(column.Title, strings.TrimSpace(needle)) {
			return i
		}
	}
	return -1
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func matchColumnToken(columns []config.Column, value string) ([]string, bool, error) {
	explicit := strings.HasPrefix(strings.TrimSpace(value), "@")
	value = strings.TrimPrefix(strings.TrimSpace(value), "@")
	ids := splitCSV(value)
	if len(ids) == 0 {
		if explicit {
			return nil, false, errors.New("column selector after @ cannot be empty")
		}
		return nil, false, nil
	}
	for _, id := range ids {
		if findColumn(columns, id) < 0 {
			if explicit {
				return nil, false, fmt.Errorf("column %q not found", id)
			}
			return nil, false, nil
		}
	}
	return ids, true, nil
}

func removeFold(values []string, needle string) []string {
	out := values[:0]
	for _, value := range values {
		if !strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(needle)) {
			out = append(out, value)
		}
	}
	return out
}

func cloneConfig(cfg config.Config) config.Config {
	out := cfg
	out.Columns = make([]config.Column, len(cfg.Columns))
	for i, column := range cfg.Columns {
		out.Columns[i] = column
		out.Columns[i].Include = append([]string(nil), column.Include...)
		out.Columns[i].Exclude = append([]string(nil), column.Exclude...)
	}
	out.Feeds = make([]config.Feed, len(cfg.Feeds))
	for i, feed := range cfg.Feeds {
		out.Feeds[i] = feed
		out.Feeds[i].Columns = append([]string(nil), feed.Columns...)
		out.Feeds[i].Tags = append([]string(nil), feed.Tags...)
	}
	return out
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

// HelpLines is shared by the command overlay and :help output.
func HelpLines() []string {
	return []string{
		"feed add <site-or-feed-url> [name…] [column]  discover, add, and refresh",
		"feed remove <name-or-url>          remove a source",
		"feed list                          show configured sources",
		"feed errors                        show source failures",
		"column add <id> <title> [topics…]  create a deck column",
		"column remove <id>                 remove a deck column",
		"column list                        show columns and filters",
		"topic add|remove <column> <term…>  edit column matching",
		"refresh <duration>|now             set cadence or fetch now",
		"drift <duration>                   set ambient card cadence",
		"cards auto|1-8                     set visible cards per column",
		"theme aurora|ember|ocean|mono      choose a color system",
		"mode deck|stream                   choose the layout",
		"images on|off                      toggle image mosaics",
		"ambient on|off                     toggle idle motion",
		"reload                             reread config from disk",
	}
}
