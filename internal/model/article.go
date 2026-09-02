package model

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Article is the normalized representation shared by ingestion, cache,
// matching, and rendering. Remote text must be sanitized before assignment.
type Article struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Source      string    `json:"source"`
	FeedName    string    `json:"feed_name"`
	Excerpt     string    `json:"excerpt"`
	ImageURL    string    `json:"image_url,omitempty"`
	Categories  []string  `json:"categories,omitempty"`
	ColumnHints []string  `json:"column_hints,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	FetchedAt   time.Time `json:"fetched_at"`
}

func StableID(guid, rawURL, title string, published time.Time) string {
	if guid = strings.TrimSpace(guid); guid != "" {
		return "guid:" + guid
	}
	if canonical := CanonicalURL(rawURL); canonical != "" {
		return "url:" + canonical
	}
	payload := strings.ToLower(strings.TrimSpace(title)) + "\x00" + published.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(payload))
	return "hash:" + hex.EncodeToString(sum[:12])
}

func CanonicalURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func NormalizeStrings(values []string) []string {
	seen := make(map[string]string, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; !exists {
			seen[key] = value
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}
