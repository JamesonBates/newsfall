package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"newsfall/internal/config"
)

func TestFetcherNormalizesRSSAtomJSONAndIsolatesFailures(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rss", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:media="http://search.yahoo.com/mrss/"><channel><title>RSS Wire</title><item><guid>rss-1</guid><title>RSS &amp; launch</title><link>`+server.URL+`/story?utm_source=rss</link><description><![CDATA[<p>Hello <b>reader</b></p><script>bad()</script>]]></description><content:encoded><![CDATA[<p>Longer body</p>]]></content:encoded><pubDate>Wed, 02 Sep 2026 12:00:00 GMT</pubDate><category>Technology</category><media:content url="/hero.jpg" type="image/jpeg" /></item></channel></rss>`)
	})
	mux.HandleFunc("/atom", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/"><title>Atom Wire</title><entry><id>atom-1</id><title>Atom title</title><link href="/atom-story"/><updated>2026-09-02T13:00:00Z</updated><summary type="html">&lt;p&gt;Atom summary&lt;/p&gt;</summary><category term="Research"/><media:thumbnail url="/thumb.png"/></entry></feed>`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/feed+json")
		fmt.Fprint(w, `{"version":"https://jsonfeed.org/version/1.1","title":"JSON Wire","items":[{"id":"json-1","url":"`+server.URL+`/json-story","title":"JSON title","summary":"JSON summary","image":"/json.png","date_published":"2026-09-02T14:00:00Z","tags":["Games"]}]}`)
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "definitely not a feed")
	})

	fetcher := NewFetcher()
	fetcher.Client = &http.Client{Timeout: 2 * time.Second}
	result := fetcher.Fetch(context.Background(), []config.Feed{
		{Name: "RSS", URL: server.URL + "/rss", Columns: []string{"ai"}, Tags: []string{"starter"}},
		{Name: "Atom", URL: server.URL + "/atom", Columns: []string{"ai"}},
		{Name: "JSON", URL: server.URL + "/json", Columns: []string{"games"}},
		{Name: "Bad", URL: server.URL + "/bad"},
	})
	if len(result.Errors) != 1 || result.Errors[0].Feed != "Bad" {
		t.Fatalf("errors = %#v", result.Errors)
	}
	if len(result.Articles) != 3 {
		t.Fatalf("articles = %d: %#v", len(result.Articles), result.Articles)
	}
	byTitle := make(map[string]int)
	for i, article := range result.Articles {
		byTitle[article.Title] = i
	}
	rss := result.Articles[byTitle["RSS & launch"]]
	if rss.Excerpt != "Longer body" || rss.ImageURL != server.URL+"/hero.jpg" {
		t.Fatalf("RSS = %#v", rss)
	}
	if strings.Contains(rss.URL, "utm_source") || len(rss.ColumnHints) != 1 || len(rss.Categories) != 2 {
		t.Fatalf("RSS metadata = %#v", rss)
	}
	atom := result.Articles[byTitle["Atom title"]]
	if atom.URL != server.URL+"/atom-story" || atom.ImageURL != server.URL+"/thumb.png" || atom.Excerpt != "Atom summary" {
		t.Fatalf("Atom = %#v", atom)
	}
	jsonArticle := result.Articles[byTitle["JSON title"]]
	if jsonArticle.ImageURL != server.URL+"/json.png" || jsonArticle.Source != "JSON" {
		t.Fatalf("JSON = %#v", jsonArticle)
	}
}

func TestFetcherHonorsCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := NewFetcher().Fetch(ctx, []config.Feed{{Name: "Slow", URL: server.URL}})
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestFetcherDiscoversLinkedFeedFromOrdinaryWebsiteURL(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head>
			<link title="Example RSS" href="/news.xml" type="application/rss+xml" rel="alternate">
		</head><body>Example</body></html>`)
	})
	mux.HandleFunc("/news.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Example Dispatch</title><item><guid>one</guid><title>Discovered story</title><link>`+server.URL+`/story</link></item></channel></rss>`)
	})

	result := NewFetcher().Fetch(context.Background(), []config.Feed{{Name: "Example", URL: server.URL + "/", Columns: []string{"ai"}}})
	if len(result.Errors) != 0 || len(result.Articles) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Articles[0].Title != "Discovered story" || result.Articles[0].ColumnHints[0] != "ai" {
		t.Fatalf("article = %#v", result.Articles[0])
	}
	if len(result.Resolutions) != 1 || result.Resolutions[0].From != server.URL+"/" || result.Resolutions[0].To != server.URL+"/news.xml" {
		t.Fatalf("resolutions = %#v", result.Resolutions)
	}
}

func TestFetcherTriesCommonFeedPathsWhenHomepageHasNoLinkTag(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/publication", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><body>Publication</body></html>`)
	})
	mux.HandleFunc("/feed", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Fallback Feed</title><entry><id>one</id><title>Fallback story</title><link href="/story"/></entry></feed>`)
	})

	result := NewFetcher().Fetch(context.Background(), []config.Feed{{Name: "Fallback", URL: server.URL + "/publication"}})
	if len(result.Errors) != 0 || len(result.Articles) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Resolutions) != 1 || result.Resolutions[0].To != server.URL+"/feed" {
		t.Fatalf("resolutions = %#v", result.Resolutions)
	}
}

func TestFetcherExplainsWhenWebsiteHasNoUsableFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><body>No feed here</body></html>`)
	}))
	defer server.Close()

	result := NewFetcher().Fetch(context.Background(), []config.Feed{{Name: "No Feed", URL: server.URL}})
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %#v", result.Errors)
	}
	message := result.Errors[0].Error()
	for _, want := range []string{"No Feed", server.URL, "webpage", "RSS", "feed URL"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
}

func TestFetcherBoundsTheEntireDiscoveryAttemptPerSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><html><body>Slow candidates</body></html>`)
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	fetcher := NewFetcher()
	fetcher.SourceTimeout = 80 * time.Millisecond
	started := time.Now()
	result := fetcher.Fetch(context.Background(), []config.Feed{{Name: "Slow Discovery", URL: server.URL}})
	elapsed := time.Since(started)
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %#v", result.Errors)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("discovery took %s despite source timeout", elapsed)
	}
}
