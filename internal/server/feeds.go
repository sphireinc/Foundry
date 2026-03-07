package server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/content"
)

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr,omitempty"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "site graph unavailable", http.StatusServiceUnavailable)
		return
	}

	posts := make([]*content.Document, 0)
	for _, doc := range graph.ByType["post"] {
		if doc.Draft {
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

	limit := s.cfg.Feed.RSSLimit
	if limit > 0 && len(posts) > limit {
		posts = posts[:limit]
	}

	baseURL := strings.TrimRight(s.cfg.BaseURL, "/")
	items := make([]rssItem, 0, len(posts))
	var lastBuild string

	for _, post := range posts {
		item := rssItem{
			Title:       post.Title,
			Link:        baseURL + post.URL,
			GUID:        baseURL + post.URL,
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

	payload := rss{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Channel: rssChannel{
			Title:         s.cfg.Feed.RSSTitle,
			Link:          baseURL,
			Description:   s.cfg.Feed.RSSDescription,
			LastBuildDate: lastBuild,
			Items:         items,
		},
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(payload)
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "site graph unavailable", http.StatusServiceUnavailable)
		return
	}

	baseURL := strings.TrimRight(s.cfg.BaseURL, "/")
	urls := make([]sitemapURL, 0, len(graph.Documents))

	for _, doc := range graph.Documents {
		if doc.Draft {
			continue
		}

		entry := sitemapURL{
			Loc: baseURL + doc.URL,
		}
		if doc.UpdatedAt != nil {
			entry.LastMod = doc.UpdatedAt.Format("2006-01-02")
		} else if doc.Date != nil {
			entry.LastMod = doc.Date.Format("2006-01-02")
		}
		urls = append(urls, entry)
	}

	payload := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	defer func(enc *xml.Encoder) {
		err := enc.Close()
		if err != nil {
			_ = fmt.Errorf("error closing XML encoder: %v", err)
		}
	}(enc)
	enc.Indent("", "  ")
	_ = enc.Encode(payload)
}
