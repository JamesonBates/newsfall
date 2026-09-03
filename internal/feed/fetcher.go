package feed

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"newsfall/internal/config"
	"newsfall/internal/content"
	"newsfall/internal/model"
)

const userAgent = "Newsfall/0.1 (+terminal feed reader)"

type FeedError struct {
	Feed string
	URL  string
	Err  error
}

func (e FeedError) Error() string {
	if strings.TrimSpace(e.URL) == "" {
		return fmt.Sprintf("%s: %v", e.Feed, e.Err)
	}
	return fmt.Sprintf("%s [%s]: %v", e.Feed, e.URL, e.Err)
}

// Resolution records a website or redirect URL that Newsfall resolved to the
// actual feed endpoint. The runtime persists it so discovery is paid only once.
type Resolution struct {
	Feed string
	From string
	To   string
}

type Result struct {
	Articles    []model.Article
	Errors      []FeedError
	Resolutions []Resolution
	Started     time.Time
	Finished    time.Time
}

type Fetcher struct {
	Client        *http.Client
	MaxConcurrent int
	MaxBodyBytes  int64
	SourceTimeout time.Duration
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		Client:        &http.Client{Timeout: 12 * time.Second},
		MaxConcurrent: 6,
		MaxBodyBytes:  8 << 20,
		SourceTimeout: 15 * time.Second,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, feeds []config.Feed) Result {
	result := Result{Started: time.Now()}
	if len(feeds) == 0 {
		result.Finished = time.Now()
		return result
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	maxConcurrent := f.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 6
	}
	maxBody := f.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 8 << 20
	}
	sourceTimeout := f.SourceTimeout
	if sourceTimeout <= 0 {
		sourceTimeout = 15 * time.Second
	}
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, definition := range feeds {
		definition := definition
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				result.Errors = append(result.Errors, FeedError{Feed: definition.Name, URL: definition.URL, Err: ctx.Err()})
				mu.Unlock()
				return
			}
			sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
			articles, resolvedURL, err := fetchOne(sourceCtx, client, maxBody, definition)
			cancel()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Errors = append(result.Errors, FeedError{Feed: definition.Name, URL: definition.URL, Err: err})
				return
			}
			result.Articles = append(result.Articles, articles...)
			if !sameURL(definition.URL, resolvedURL) {
				result.Resolutions = append(result.Resolutions, Resolution{Feed: definition.Name, From: definition.URL, To: resolvedURL})
			}
		}()
	}
	wg.Wait()
	result.Articles = content.Deduplicate(result.Articles)
	result.Finished = time.Now()
	return result
}

func fetchOne(ctx context.Context, client *http.Client, maxBody int64, definition config.Feed) ([]model.Article, string, error) {
	parsed, resolvedURL, err := loadParsedFeed(ctx, client, maxBody, definition.URL)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	feedName := strings.TrimSpace(definition.Name)
	if feedName == "" {
		feedName = content.CleanText(parsed.Title)
	}
	articles := make([]model.Article, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		title := content.CleanText(item.Title)
		if title == "" {
			continue
		}
		link := resolve(resolvedURL, item.URL)
		if canonical := model.CanonicalURL(link); canonical != "" {
			link = canonical
		}
		published := item.Published
		if published.IsZero() {
			published = now
		}
		excerpt := content.Excerpt(item.Excerpt, 500)
		imageURL := resolve(firstNonEmpty(link, resolvedURL), item.ImageURL)
		source := content.CleanText(item.Source)
		if source == "" {
			source = feedName
		}
		categories := model.NormalizeStrings(append(append([]string{}, item.Categories...), definition.Tags...))
		articles = append(articles, model.Article{
			ID:          model.StableID(item.GUID, link, title, published),
			Title:       title,
			URL:         link,
			Source:      source,
			FeedName:    feedName,
			Excerpt:     excerpt,
			ImageURL:    imageURL,
			Categories:  categories,
			ColumnHints: model.NormalizeStrings(definition.Columns),
			PublishedAt: published.UTC(),
			FetchedAt:   now,
		})
	}
	return articles, resolvedURL, nil
}
