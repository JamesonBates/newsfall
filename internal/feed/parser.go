package feed

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	stdhtml "html"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type parsedFeed struct {
	Title string
	Items []parsedItem
}

type parsedItem struct {
	GUID       string
	Title      string
	URL        string
	Excerpt    string
	ImageURL   string
	Source     string
	Categories []string
	Published  time.Time
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
	Items   []rssItem  `xml:"item"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	GUID          string         `xml:"guid"`
	Title         string         `xml:"title"`
	Link          string         `xml:"link"`
	Description   string         `xml:"description"`
	Encoded       string         `xml:"encoded"`
	PubDate       string         `xml:"pubDate"`
	Date          string         `xml:"date"`
	Source        string         `xml:"source"`
	Categories    []string       `xml:"category"`
	Enclosures    []enclosure    `xml:"enclosure"`
	MediaContents []mediaElement `xml:"content"`
	Thumbnails    []mediaElement `xml:"thumbnail"`
}

type enclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaElement struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
}

type atomDocument struct {
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Links      []atomLink     `xml:"link"`
	Summary    atomText       `xml:"summary"`
	Content    atomContent    `xml:"content"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Categories []atomCategory `xml:"category"`
	Thumbnail  []mediaElement `xml:"thumbnail"`
	MediaGroup struct {
		Contents   []mediaElement `xml:"content"`
		Thumbnails []mediaElement `xml:"thumbnail"`
	} `xml:"group"`
}

type atomText struct {
	Inner string `xml:",innerxml"`
}

type atomContent struct {
	Inner  string `xml:",innerxml"`
	Source string `xml:"src,attr"`
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type jsonDocument struct {
	Title string     `json:"title"`
	Items []jsonItem `json:"items"`
}

type jsonItem struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	ExternalURL   string   `json:"external_url"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	ContentText   string   `json:"content_text"`
	ContentHTML   string   `json:"content_html"`
	Image         string   `json:"image"`
	BannerImage   string   `json:"banner_image"`
	DatePublished string   `json:"date_published"`
	DateModified  string   `json:"date_modified"`
	Tags          []string `json:"tags"`
}

var imageTag = regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']?([^"' >]+)`)

func parseDocument(data []byte) (parsedFeed, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return parsedFeed{}, errors.New("empty feed")
	}
	if trimmed[0] == '{' {
		return parseJSON(trimmed)
	}
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(trimmed, &root); err != nil {
		return parsedFeed{}, err
	}
	switch strings.ToLower(root.XMLName.Local) {
	case "feed":
		return parseAtom(trimmed)
	case "rss", "rdf":
		return parseRSS(trimmed)
	default:
		return parsedFeed{}, errors.New("unrecognized feed format")
	}
}

func parseRSS(data []byte) (parsedFeed, error) {
	var doc rssDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return parsedFeed{}, err
	}
	items := doc.Channel.Items
	if len(items) == 0 {
		items = doc.Items
	}
	out := parsedFeed{Title: strings.TrimSpace(doc.Channel.Title)}
	for _, item := range items {
		excerpt := item.Encoded
		if strings.TrimSpace(excerpt) == "" {
			excerpt = item.Description
		}
		published := parseDate(firstNonEmpty(item.PubDate, item.Date))
		imageURL := firstImage(item.MediaContents, item.Thumbnails)
		if imageURL == "" {
			for _, enclosure := range item.Enclosures {
				if strings.HasPrefix(strings.ToLower(enclosure.Type), "image/") {
					imageURL = enclosure.URL
					break
				}
			}
		}
		if imageURL == "" {
			imageURL = imageFromHTML(excerpt)
		}
		out.Items = append(out.Items, parsedItem{
			GUID: item.GUID, Title: item.Title, URL: item.Link, Excerpt: excerpt,
			ImageURL: imageURL, Source: item.Source, Categories: item.Categories, Published: published,
		})
	}
	return out, nil
}

func parseAtom(data []byte) (parsedFeed, error) {
	var doc atomDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return parsedFeed{}, err
	}
	out := parsedFeed{Title: strings.TrimSpace(doc.Title)}
	for _, entry := range doc.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		excerpt := entry.Summary.Inner
		if strings.TrimSpace(excerpt) == "" {
			excerpt = entry.Content.Inner
		}
		imageURL := firstImage(entry.Thumbnail, entry.MediaGroup.Thumbnails, entry.MediaGroup.Contents)
		if imageURL == "" && (strings.HasPrefix(strings.ToLower(entry.Content.Type), "image/") || strings.EqualFold(entry.Content.Medium, "image")) {
			imageURL = firstNonEmpty(entry.Content.URL, entry.Content.Source)
		}
		if imageURL == "" {
			imageURL = imageFromHTML(excerpt)
		}
		categories := make([]string, 0, len(entry.Categories))
		for _, category := range entry.Categories {
			categories = append(categories, category.Term)
		}
		out.Items = append(out.Items, parsedItem{
			GUID: entry.ID, Title: entry.Title, URL: link, Excerpt: excerpt, ImageURL: imageURL,
			Categories: categories, Published: parseDate(firstNonEmpty(entry.Published, entry.Updated)),
		})
	}
	return out, nil
}

func parseJSON(data []byte) (parsedFeed, error) {
	var doc jsonDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return parsedFeed{}, err
	}
	out := parsedFeed{Title: strings.TrimSpace(doc.Title)}
	for _, item := range doc.Items {
		excerpt := firstNonEmpty(item.Summary, item.ContentText, item.ContentHTML)
		out.Items = append(out.Items, parsedItem{
			GUID: item.ID, Title: item.Title, URL: firstNonEmpty(item.URL, item.ExternalURL), Excerpt: excerpt,
			ImageURL:   firstNonEmpty(item.Image, item.BannerImage, imageFromHTML(item.ContentHTML)),
			Categories: item.Tags, Published: parseDate(firstNonEmpty(item.DatePublished, item.DateModified)),
		})
	}
	return out, nil
}

func parseDate(value string) time.Time {
	value = strings.TrimSpace(value)
	formats := []string{
		time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123,
		time.RFC822Z, time.RFC822, time.RFC850, time.ANSIC,
		"Mon, 02 Jan 2006 15:04:05 MST", "2006-01-02 15:04:05 -0700",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstImage(groups ...[]mediaElement) string {
	for _, group := range groups {
		for _, element := range group {
			if element.URL == "" {
				continue
			}
			if element.Medium == "" && element.Type == "" || strings.EqualFold(element.Medium, "image") || strings.HasPrefix(strings.ToLower(element.Type), "image/") {
				return element.URL
			}
		}
	}
	return ""
}

func imageFromHTML(value string) string {
	match := imageTag.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return stdhtml.UnescapeString(strings.TrimSpace(match[1]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolve(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}
