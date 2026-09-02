package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const currentVersion = 1

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

type Config struct {
	Version      int      `json:"version"`
	Refresh      string   `json:"refresh"`
	Drift        string   `json:"drift"`
	Mode         string   `json:"mode"`
	Theme        string   `json:"theme"`
	Images       bool     `json:"images"`
	Ambient      bool     `json:"ambient"`
	MaxItems     int      `json:"max_items"`
	MaxPerColumn int      `json:"max_per_column"`
	Columns      []Column `json:"columns"`
	Feeds        []Feed   `json:"feeds"`
}

type Feed struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Columns []string `json:"columns,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type Column struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
	Accent  string   `json:"accent,omitempty"`
}

func Default() Config {
	return Config{
		Version:      currentVersion,
		Refresh:      "5m",
		Drift:        "12s",
		Mode:         "deck",
		Theme:        "aurora",
		Images:       true,
		Ambient:      true,
		MaxItems:     240,
		MaxPerColumn: 80,
		Columns: []Column{
			{ID: "ai", Title: "AI + TECH", Include: []string{"AI", "artificial intelligence", "OpenAI", "Anthropic", "LLM", "agent", "ChatGPT", "Codex", "model"}, Accent: "#8B5CF6"},
			{ID: "machines", Title: "MACHINES", Accent: "#22D3EE"},
			{ID: "games", Title: "GAMES + CULTURE", Accent: "#F97316"},
		},
		Feeds: []Feed{
			{Name: "Hacker News", URL: "https://hnrss.org/frontpage", Columns: []string{"ai"}, Tags: []string{"technology"}},
			{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml", Columns: []string{"ai"}, Tags: []string{"technology"}},
			{Name: "BMWBLOG", URL: "https://www.bmwblog.com/feed/", Columns: []string{"machines"}, Tags: []string{"automotive", "BMW"}},
			{Name: "The Drive", URL: "https://www.thedrive.com/feed", Columns: []string{"machines"}, Tags: []string{"automotive"}},
			{Name: "IGN", URL: "https://feeds.feedburner.com/ign/all", Columns: []string{"games"}, Tags: []string{"games"}},
			{Name: "Polygon", URL: "https://www.polygon.com/rss/index.xml", Columns: []string{"games"}, Tags: []string{"games", "culture"}},
		},
	}
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.Version == 0 {
		cfg.Version = currentVersion
	}
	if cfg.Refresh == "" {
		cfg.Refresh = "5m"
	}
	if cfg.Drift == "" {
		cfg.Drift = "12s"
	}
	if cfg.Mode == "" {
		cfg.Mode = "deck"
	}
	if cfg.Theme == "" {
		cfg.Theme = "aurora"
	}
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = 240
	}
	if cfg.MaxPerColumn <= 0 {
		cfg.MaxPerColumn = 80
	}
	if len(cfg.Columns) < 1 || len(cfg.Columns) > 3 {
		return fmt.Errorf("Newsfall requires between 1 and 3 columns; got %d", len(cfg.Columns))
	}
	if cfg.Mode != "deck" && cfg.Mode != "stream" {
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
	if !validTheme(cfg.Theme) {
		return fmt.Errorf("unsupported theme %q", cfg.Theme)
	}
	if d, err := time.ParseDuration(cfg.Refresh); err != nil || d < 10*time.Second {
		return fmt.Errorf("refresh must be at least 10s: %q", cfg.Refresh)
	}
	if d, err := time.ParseDuration(cfg.Drift); err != nil || d < time.Second {
		return fmt.Errorf("drift must be at least 1s: %q", cfg.Drift)
	}

	columnIDs := make(map[string]struct{}, len(cfg.Columns))
	for i := range cfg.Columns {
		column := &cfg.Columns[i]
		column.Title = strings.TrimSpace(column.Title)
		column.ID = slug(column.ID)
		if column.ID == "" {
			column.ID = slug(column.Title)
		}
		if column.Title == "" {
			column.Title = strings.ToUpper(strings.ReplaceAll(column.ID, "-", " "))
		}
		if column.ID == "" {
			return fmt.Errorf("column %d needs an id or title", i+1)
		}
		if _, exists := columnIDs[column.ID]; exists {
			return fmt.Errorf("duplicate column id %q", column.ID)
		}
		columnIDs[column.ID] = struct{}{}
		column.Include = normalizeTerms(column.Include)
		column.Exclude = normalizeTerms(column.Exclude)
		column.Accent = strings.TrimSpace(column.Accent)
	}

	feedURLs := make(map[string]struct{}, len(cfg.Feeds))
	for i := range cfg.Feeds {
		feed := &cfg.Feeds[i]
		feed.URL = strings.TrimSpace(feed.URL)
		u, err := url.Parse(feed.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("feed %d has invalid URL %q", i+1, feed.URL)
		}
		key := strings.ToLower(u.Scheme + "://" + u.Host + strings.TrimRight(u.EscapedPath(), "/"))
		if u.RawQuery != "" {
			key += "?" + u.RawQuery
		}
		if _, exists := feedURLs[key]; exists {
			return fmt.Errorf("duplicate feed URL %q", feed.URL)
		}
		feedURLs[key] = struct{}{}
		feed.Name = strings.TrimSpace(feed.Name)
		if feed.Name == "" {
			feed.Name = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		}
		feed.Columns = normalizeIDs(feed.Columns)
		for _, id := range feed.Columns {
			if _, exists := columnIDs[id]; !exists {
				return fmt.Errorf("feed %q references unknown column %q", feed.Name, id)
			}
		}
		feed.Tags = normalizeTerms(feed.Tags)
	}
	return nil
}

func RefreshDuration(cfg Config) time.Duration {
	if d, err := time.ParseDuration(cfg.Refresh); err == nil && d > 0 {
		return d
	}
	return 5 * time.Minute
}

func DriftDuration(cfg Config) time.Duration {
	if d, err := time.ParseDuration(cfg.Drift); err == nil && d > 0 {
		return d
	}
	return 12 * time.Second
}

func Load(path string) (Config, error) {
	if path == "" {
		path = ConfigPath()
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := Save(path, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := Validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		path = ConfigPath()
	}
	if err := Validate(&cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".newsfall-*.json")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temporary config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func ConfigPath() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "newsfall", "config.json")
	}
	if runtime.GOOS == "windows" {
		if base, err := os.UserConfigDir(); err == nil {
			return filepath.Join(base, "newsfall", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "newsfall-config.json"
	}
	return filepath.Join(home, ".config", "newsfall", "config.json")
}

func DataPath() string {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "newsfall", "articles.json")
	}
	if runtime.GOOS == "windows" {
		if base, err := os.UserCacheDir(); err == nil {
			return filepath.Join(base, "newsfall", "articles.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "newsfall-articles.json"
	}
	return filepath.Join(home, ".local", "share", "newsfall", "articles.json")
}

func validTheme(theme string) bool {
	switch strings.ToLower(theme) {
	case "aurora", "ember", "ocean", "mono":
		return true
	default:
		return false
	}
}

func slug(value string) string {
	value = nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-")
	return strings.Trim(value, "-")
}

func normalizeIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = slug(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeTerms(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
