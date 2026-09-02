package content

import (
	"sort"
	"strings"
	"time"

	"newsfall/internal/config"
	"newsfall/internal/model"
)

func Deduplicate(items []model.Article) []model.Article {
	byID := make(map[string]model.Article, len(items))
	for _, item := range items {
		if item.ID == "" {
			item.ID = model.StableID("", item.URL, item.Title, item.PublishedAt)
		}
		current, exists := byID[item.ID]
		if !exists {
			item.Categories = model.NormalizeStrings(item.Categories)
			item.ColumnHints = model.NormalizeStrings(item.ColumnHints)
			byID[item.ID] = item
			continue
		}
		newer, older := current, item
		if articleTime(item).After(articleTime(current)) {
			newer, older = item, current
		}
		if newer.Excerpt == "" {
			newer.Excerpt = older.Excerpt
		}
		if newer.ImageURL == "" {
			newer.ImageURL = older.ImageURL
		}
		if newer.Source == "" {
			newer.Source = older.Source
		}
		if newer.FeedName == "" {
			newer.FeedName = older.FeedName
		}
		newer.Categories = model.NormalizeStrings(append(newer.Categories, older.Categories...))
		newer.ColumnHints = model.NormalizeStrings(append(newer.ColumnHints, older.ColumnHints...))
		byID[item.ID] = newer
	}
	out := make([]model.Article, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	SortNewest(out)
	return out
}

func SortNewest(items []model.Article) {
	sort.SliceStable(items, func(i, j int) bool {
		ti, tj := articleTime(items[i]), articleTime(items[j])
		if ti.Equal(tj) {
			return items[i].ID < items[j].ID
		}
		return ti.After(tj)
	})
}

func Matches(article model.Article, column config.Column) bool {
	searchable := strings.ToLower(strings.Join([]string{
		article.Title,
		article.Excerpt,
		article.Source,
		article.FeedName,
		strings.Join(article.Categories, " "),
	}, " "))
	for _, term := range column.Exclude {
		if strings.Contains(searchable, strings.ToLower(term)) {
			return false
		}
	}
	if len(article.ColumnHints) > 0 {
		found := false
		for _, hint := range article.ColumnHints {
			if strings.EqualFold(hint, column.ID) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(column.Include) == 0 {
		return true
	}
	for _, term := range column.Include {
		if strings.Contains(searchable, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func Assign(items []model.Article, columns []config.Column, maxPerColumn int) map[string][]model.Article {
	ordered := append([]model.Article(nil), items...)
	SortNewest(ordered)
	out := make(map[string][]model.Article, len(columns))
	for _, column := range columns {
		for _, item := range ordered {
			if !Matches(item, column) {
				continue
			}
			out[column.ID] = append(out[column.ID], item)
			if maxPerColumn > 0 && len(out[column.ID]) >= maxPerColumn {
				break
			}
		}
	}
	return out
}

func articleTime(item model.Article) time.Time {
	if !item.PublishedAt.IsZero() {
		return item.PublishedAt
	}
	return item.FetchedAt
}
