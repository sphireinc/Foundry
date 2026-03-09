package feedcmd

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
)

type command struct{}

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (command) Name() string {
	return "feed"
}

func (command) Summary() string {
	return "Build and validate RSS and sitemap output"
}

func (command) Group() string {
	return "feed commands"
}

func (command) Details() []string {
	return []string{
		"foundry feed build",
		"foundry feed validate",
	}
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry feed [build|validate]")
	}

	switch args[2] {
	case "build":
		return runBuild(cfg)
	case "validate":
		return runValidate(cfg)
	}

	return fmt.Errorf("unknown feed subcommand: %s", args[2])
}

func runBuild(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	rssXML, sitemapXML, err := buildFeeds(cfg, graph)
	if err != nil {
		return err
	}

	rssTarget := filepath.Join(cfg.PublicDir, strings.TrimPrefix(cfg.Feed.RSSPath, "/"))
	sitemapTarget := filepath.Join(cfg.PublicDir, strings.TrimPrefix(cfg.Feed.SitemapPath, "/"))

	if err := os.MkdirAll(filepath.Dir(rssTarget), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sitemapTarget), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(rssTarget, rssXML, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(sitemapTarget, sitemapXML, 0o644); err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", rssTarget)
	fmt.Printf("wrote %s\n", sitemapTarget)
	return nil
}

func runValidate(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	rssXML, sitemapXML, err := buildFeeds(cfg, graph)
	if err != nil {
		return err
	}

	var rssDoc rss
	if err := xml.Unmarshal(rssXML, &rssDoc); err != nil {
		return fmt.Errorf("rss validation failed: %w", err)
	}

	var sitemapDoc sitemapURLSet
	if err := xml.Unmarshal(sitemapXML, &sitemapDoc); err != nil {
		return fmt.Errorf("sitemap validation failed: %w", err)
	}

	fmt.Printf("feed validation OK (rss items: %d, sitemap urls: %d)\n", len(rssDoc.Channel.Items), len(sitemapDoc.URLs))
	return nil
}

func buildFeeds(cfg *config.Config, graph *content.SiteGraph) ([]byte, []byte, error) {
	docs := make([]*content.Document, 0)
	for _, doc := range graph.Documents {
		if doc.Draft {
			continue
		}
		docs = append(docs, doc)
	}

	sort.Slice(docs, func(i, j int) bool {
		di, dj := docs[i].Date, docs[j].Date
		switch {
		case di != nil && dj != nil:
			return di.After(*dj)
		case di != nil:
			return true
		case dj != nil:
			return false
		default:
			return docs[i].URL < docs[j].URL
		}
	})

	limit := cfg.Feed.RSSLimit
	if limit <= 0 || limit > len(docs) {
		limit = len(docs)
	}

	items := make([]rssItem, 0, limit)
	for _, doc := range docs[:limit] {
		item := rssItem{
			Title:       doc.Title,
			Link:        absoluteURL(cfg.BaseURL, doc.URL),
			Description: doc.Summary,
		}
		if doc.Date != nil {
			item.PubDate = doc.Date.Format(time.RFC1123Z)
		}
		items = append(items, item)
	}

	rssDoc := rss{
		Version: "2.0",
		Channel: rssChannel{
			Title:       choose(cfg.Feed.RSSTitle, cfg.Title),
			Link:        strings.TrimRight(cfg.BaseURL, "/"),
			Description: choose(cfg.Feed.RSSDescription, cfg.Title),
			Items:       items,
		},
	}

	rssXML, err := xml.MarshalIndent(rssDoc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	rssXML = append([]byte(xml.Header), rssXML...)

	sitemapDoc := sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  make([]sitemapURL, 0, len(docs)),
	}
	for _, doc := range docs {
		u := sitemapURL{
			Loc: absoluteURL(cfg.BaseURL, doc.URL),
		}
		if doc.Date != nil {
			u.LastMod = doc.Date.Format("2006-01-02")
		}
		sitemapDoc.URLs = append(sitemapDoc.URLs, u)
	}

	sitemapXML, err := xml.MarshalIndent(sitemapDoc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	sitemapXML = append([]byte(xml.Header), sitemapXML...)

	return rssXML, sitemapXML, nil
}

func loadGraph(cfg *config.Config) (*content.SiteGraph, error) {
	pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return nil, err
	}
	if err := pm.OnConfigLoaded(cfg); err != nil {
		return nil, err
	}

	loader := content.NewLoader(cfg, pm, false)
	graph, err := loader.Load(context.Background())
	if err != nil {
		return nil, err
	}

	resolver := router.NewResolver(cfg)
	if err := resolver.AssignURLs(graph); err != nil {
		return nil, err
	}
	if err := pm.OnRoutesAssigned(graph); err != nil {
		return nil, err
	}

	return graph, nil
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

func init() {
	registry.Register(command{})
}
