package feed

import (
	"context"
	"fmt"
	"io"
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

func (e FeedError) Error() string { return fmt.Sprintf("%s: %v", e.Feed, e.Err) }

type Result struct {
	Articles []model.Article
	Errors   []FeedError
	Started  time.Time
	Finished time.Time
}

type Fetcher struct {
	Client        *http.Client
	MaxConcurrent int
	MaxBodyBytes  int64
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		Client:        &http.Client{Timeout: 12 * time.Second},
		MaxConcurrent: 6,
		MaxBodyBytes:  8 << 20,
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
			articles, err := fetchOne(ctx, client, maxBody, definition)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Errors = append(result.Errors, FeedError{Feed: definition.Name, URL: definition.URL, Err: err})
				return
			}
			result.Articles = append(result.Articles, articles...)
		}()
	}
	wg.Wait()
	result.Articles = content.Deduplicate(result.Articles)
	result.Finished = time.Now()
	return result
}

func fetchOne(ctx context.Context, client *http.Client, maxBody int64, definition config.Feed) ([]model.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, definition.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/json, text/xml, application/xml;q=0.9, */*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBody {
		return nil, fmt.Errorf("feed exceeds %d bytes", maxBody)
	}
	parsed, err := parseDocument(body)
	if err != nil {
		return nil, err
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
		link := resolve(definition.URL, item.URL)
		if canonical := model.CanonicalURL(link); canonical != "" {
			link = canonical
		}
		published := item.Published
		if published.IsZero() {
			published = now
		}
		excerpt := content.Excerpt(item.Excerpt, 500)
		imageURL := resolve(firstNonEmpty(link, definition.URL), item.ImageURL)
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
	return articles, nil
}
