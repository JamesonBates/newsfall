package feed

import (
	"bytes"
	"context"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const maxDiscoveryCandidates = 10

var (
	linkTagPattern = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attrPattern    = regexp.MustCompile("(?is)([a-z_:][a-z0-9_:.\\-]*)\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s\"'=<>]+))")
)

type fetchedDocument struct {
	body        []byte
	contentType string
	url         string
}

func loadParsedFeed(ctx context.Context, client *http.Client, maxBody int64, sourceURL string) (parsedFeed, string, error) {
	document, err := fetchDocument(ctx, client, maxBody, sourceURL)
	if err != nil {
		return parsedFeed{}, "", err
	}
	parsed, parseErr := parseDocument(document.body)
	if parseErr == nil {
		return parsed, document.url, nil
	}
	if !looksLikeHTML(document.contentType, document.body) {
		return parsedFeed{}, "", parseErr
	}

	linked := discoverLinkedFeeds(document.url, document.body)
	candidates := append(linked, commonFeedURLs(document.url)...)
	candidates = uniqueURLs(candidates, document.url)
	if len(candidates) > maxDiscoveryCandidates {
		candidates = candidates[:maxDiscoveryCandidates]
	}
	var lastErr error
	for _, candidate := range candidates {
		candidateDocument, fetchErr := fetchDocument(ctx, client, maxBody, candidate)
		if fetchErr != nil {
			lastErr = fetchErr
			if ctx.Err() != nil {
				break
			}
			continue
		}
		candidateFeed, candidateErr := parseDocument(candidateDocument.body)
		if candidateErr != nil {
			lastErr = candidateErr
			continue
		}
		return candidateFeed, candidateDocument.url, nil
	}
	if ctx.Err() != nil {
		return parsedFeed{}, "", fmt.Errorf("feed discovery timed out: %w", ctx.Err())
	}

	if len(linked) > 0 && lastErr != nil {
		return parsedFeed{}, "", fmt.Errorf("webpage advertised a feed, but it could not be read: %v; try the site's feed URL directly", lastErr)
	}
	return parsedFeed{}, "", fmt.Errorf("webpage has no usable RSS, Atom, or JSON Feed; try the site's feed URL directly")
}

func fetchDocument(ctx context.Context, client *http.Client, maxBody int64, sourceURL string) (fetchedDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fetchedDocument{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/json, text/html;q=0.8, text/xml, application/xml;q=0.9, */*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return fetchedDocument{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fetchedDocument{}, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return fetchedDocument{}, err
	}
	if int64(len(body)) > maxBody {
		return fetchedDocument{}, fmt.Errorf("response exceeds %d bytes", maxBody)
	}
	finalURL := sourceURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return fetchedDocument{body: body, contentType: resp.Header.Get("Content-Type"), url: finalURL}, nil
}

func looksLikeHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := strings.ToLower(string(bytes.TrimSpace(body)))
	for _, prefix := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func discoverLinkedFeeds(baseURL string, body []byte) []string {
	matches := linkTagPattern.FindAll(body, -1)
	out := make([]string, 0, len(matches))
	for _, tag := range matches {
		attrs := parseHTMLAttributes(tag)
		if !hasToken(attrs["rel"], "alternate") || !isFeedMIME(attrs["type"]) {
			continue
		}
		href := strings.TrimSpace(stdhtml.UnescapeString(attrs["href"]))
		if href == "" {
			continue
		}
		if resolved := resolve(baseURL, href); resolved != "" {
			out = append(out, resolved)
		}
	}
	return uniqueURLs(out, "")
}

func parseHTMLAttributes(tag []byte) map[string]string {
	attrs := make(map[string]string)
	for _, match := range attrPattern.FindAllSubmatch(tag, -1) {
		if len(match) < 5 {
			continue
		}
		value := ""
		for _, candidate := range match[2:] {
			if len(candidate) > 0 {
				value = string(candidate)
				break
			}
		}
		attrs[strings.ToLower(string(match[1]))] = stdhtml.UnescapeString(value)
	}
	return attrs
}

func hasToken(value, expected string) bool {
	for _, token := range strings.Fields(strings.ToLower(value)) {
		if token == expected {
			return true
		}
	}
	return false
}

func isFeedMIME(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "application/rss+xml", "application/atom+xml", "application/feed+json", "application/json", "text/xml", "application/xml":
		return true
	default:
		return false
	}
}

func commonFeedURLs(pageURL string) []string {
	base, err := url.Parse(pageURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil
	}
	paths := []string{"/feed", "/feed/", "/rss", "/rss.xml", "/feed.xml", "/atom.xml", "/index.xml", "/rss/index.xml"}
	out := make([]string, 0, len(paths)+1)
	trimmedPath := strings.TrimRight(base.Path, "/")
	if trimmedPath != "" {
		candidate := *base
		candidate.Path = trimmedPath + "/feed"
		candidate.RawPath = ""
		candidate.RawQuery = ""
		candidate.Fragment = ""
		out = append(out, candidate.String())
	}
	for _, path := range paths {
		candidate := *base
		candidate.Path = path
		candidate.RawPath = ""
		candidate.RawQuery = ""
		candidate.Fragment = ""
		out = append(out, candidate.String())
	}
	return uniqueURLs(out, pageURL)
}

func uniqueURLs(values []string, skip string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || sameURL(value, skip) {
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

func sameURL(left, right string) bool {
	if strings.TrimSpace(right) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(left), "/"), strings.TrimRight(strings.TrimSpace(right), "/"))
}
