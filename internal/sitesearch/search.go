// Package sitesearch defines the public search contract shared by rendering and APIs.
package sitesearch

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sphireinc/foundry/internal/content"
)

const DefaultLimit = 20
const MaxLimit = 100

type Entry struct {
	Title      string              `json:"title"`
	URL        string              `json:"url"`
	Summary    string              `json:"summary,omitempty"`
	Snippet    string              `json:"snippet,omitempty"`
	Content    string              `json:"content,omitempty"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Layout     string              `json:"layout,omitempty"`
	Tags       []string            `json:"tags,omitempty"`
	Categories []string            `json:"categories,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}
type Options struct {
	Lang, Type string
	Limit      int
}
type Result struct {
	Query string  `json:"query"`
	Items []Entry `json:"items"`
	Total int     `json:"total"`
	Limit int     `json:"limit"`
}

func ParseLimit(value string) int {
	n, err := strconv.Atoi(value)
	if errors.Is(err, strconv.ErrRange) && !strings.HasPrefix(value, "-") {
		return MaxLimit
	}
	if err != nil || n < 1 {
		return DefaultLimit
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

var htmlTags = regexp.MustCompile(`<[^>]*>`)

func Entries(graph *content.SiteGraph) []Entry {
	out := []Entry{}
	if graph == nil {
		return out
	}
	now := time.Now()
	for _, doc := range graph.Documents {
		if !Public(doc, now) {
			continue
		}
		text := doc.RawBody
		if strings.TrimSpace(text) == "" {
			text = htmlTags.ReplaceAllString(string(doc.HTMLBody), " ")
		}
		text = strings.Join(strings.Fields(text), " ")
		out = append(out, Entry{Title: doc.Title, URL: doc.URL, Summary: doc.Summary, Snippet: Snippet(doc.Title, doc.Summary, text), Content: text, Type: doc.Type, Lang: doc.Lang, Layout: doc.Layout, Tags: doc.Taxonomies["tags"], Categories: doc.Taxonomies["categories"], Taxonomies: doc.Taxonomies})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}
func Public(doc *content.Document, now time.Time) bool {
	if doc == nil {
		return false
	}
	workflow := content.WorkflowFromFrontMatter(&content.FrontMatter{Draft: doc.Draft, Params: doc.Params}, now)
	return workflow.Status == "published"
}
func Snippet(title, summary, body string, queries ...string) string {
	text := strings.TrimSpace(summary)
	if text != "" {
		return truncate(text, 180)
	}
	text = strings.TrimSpace(body)
	if text == "" {
		return truncate(strings.TrimSpace(title), 180)
	}
	runes := []rune(text)
	start := 0
	if len(queries) > 0 && queries[0] != "" {
		lower := strings.ToLower(text)
		if position := strings.Index(lower, queries[0]); position >= 0 {
			start = utf8.RuneCountInString(lower[:position]) - 60
			if start < 0 {
				start = 0
			}
		}
	}
	end := start + 180
	if end > len(runes) {
		end = len(runes)
	}
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}
func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit])) + "..."
	}
	return text
}

func Query(entries []Entry, query string, opts Options) Result {
	query = strings.ToLower(strings.TrimSpace(query))
	limit := opts.Limit
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	result := Result{Query: query, Items: []Entry{}, Limit: limit}
	// An empty query is an invitation to search, not a browse-all request.
	if query == "" {
		return result
	}
	type scored struct {
		entry Entry
		score int
	}
	matches := []scored{}
	for _, entry := range entries {
		if opts.Lang != "" && entry.Lang != opts.Lang || opts.Type != "" && entry.Type != opts.Type {
			continue
		}
		score := 0
		for _, field := range []struct {
			text   string
			weight int
		}{{entry.Title, 6}, {entry.Summary, 4}, {entry.Content, 2}, {entry.URL, 1}} {
			if strings.Contains(strings.ToLower(field.text), query) {
				score += field.weight
			}
		}
		if score > 0 {
			entry.Snippet = Snippet(entry.Title, entry.Summary, entry.Content, query)
			matches = append(matches, scored{entry, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.entry.Title != b.entry.Title {
			return a.entry.Title < b.entry.Title
		}
		return a.entry.URL < b.entry.URL
	})
	result.Total = len(matches)
	for i, match := range matches {
		if i >= limit {
			break
		}
		result.Items = append(result.Items, match.entry)
	}
	return result
}
