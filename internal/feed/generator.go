package feed

import (
	"encoding/xml"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

type RSS struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr,omitempty"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
}

type URLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []SitemapURL `xml:"url"`
}

type SitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func Build(cfg *config.Config, graph *content.SiteGraph) ([]byte, []byte, error) {
	rssXML, err := BuildRSS(cfg, graph)
	if err != nil {
		return nil, nil, err
	}

	sitemapXML, err := BuildSitemap(cfg, graph)
	if err != nil {
		return nil, nil, err
	}

	return rssXML, sitemapXML, nil
}

func BuildRSS(cfg *config.Config, graph *content.SiteGraph) ([]byte, error) {
	posts := collectPosts(graph)

	limit := cfg.Feed.RSSLimit
	if limit > 0 && len(posts) > limit {
		posts = posts[:limit]
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	items := make([]RSSItem, 0, len(posts))
	var lastBuild string

	for _, post := range posts {
		item := RSSItem{
			Title:       post.Title,
			Link:        absoluteURL(baseURL, post.URL),
			GUID:        absoluteURL(baseURL, post.URL),
			Description: post.Summary,
		}

		if post.Date != nil {
			item.PubDate = post.Date.Format(time.RFC1123Z)
			if lastBuild == "" {
				lastBuild = item.PubDate
			}
		}

		items = append(items, item)
	}

	payload := RSS{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Channel: RSSChannel{
			Title:         choose(cfg.Feed.RSSTitle, cfg.Title),
			Link:          baseURL,
			Description:   choose(cfg.Feed.RSSDescription, cfg.Title),
			LastBuildDate: lastBuild,
			Items:         items,
		},
	}

	out, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), out...), nil
}

func BuildSitemap(cfg *config.Config, graph *content.SiteGraph) ([]byte, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	urls := make([]SitemapURL, 0, len(graph.Documents))

	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft {
			continue
		}

		entry := SitemapURL{
			Loc: absoluteURL(baseURL, doc.URL),
		}

		if doc.UpdatedAt != nil {
			entry.LastMod = doc.UpdatedAt.Format("2006-01-02")
		} else if doc.Date != nil {
			entry.LastMod = doc.Date.Format("2006-01-02")
		}

		urls = append(urls, entry)
	}

	payload := URLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	out, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), out...), nil
}

func collectPosts(graph *content.SiteGraph) []*content.Document {
	posts := make([]*content.Document, 0)

	for _, doc := range graph.ByType["post"] {
		if doc == nil || doc.Draft {
			continue
		}
		posts = append(posts, doc)
	}

	sort.Slice(posts, func(i, j int) bool {
		var ti, tj time.Time
		if posts[i].Date != nil {
			ti = *posts[i].Date
		}
		if posts[j].Date != nil {
			tj = *posts[j].Date
		}
		return ti.After(tj)
	})

	return posts
}

func absoluteURL(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if path == "" {
		return baseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
