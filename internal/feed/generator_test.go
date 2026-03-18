package feed

import (
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func TestBuildRSSAndSitemap(t *testing.T) {
	cfg, graph := testFeedGraph()

	rssXML, sitemapXML, err := Build(cfg, graph)
	if err != nil {
		t.Fatalf("build feeds: %v", err)
	}

	rss := string(rssXML)
	if !strings.Contains(rss, "<title>Feed Title</title>") || strings.Contains(rss, "Draft") {
		t.Fatalf("unexpected rss: %s", rss)
	}
	if strings.Index(rss, "Newer") > strings.Index(rss, "Older") {
		t.Fatalf("expected newer post before older post")
	}

	sitemap := string(sitemapXML)
	if !strings.Contains(sitemap, "https://example.com/posts/newer/") || strings.Contains(sitemap, "draft-post") {
		t.Fatalf("unexpected sitemap: %s", sitemap)
	}
}

func TestHelpers(t *testing.T) {
	cfg, graph := testFeedGraph()
	posts := collectPosts(graph)
	if len(posts) != 2 || posts[0].Title != "Newer" {
		t.Fatalf("unexpected collected posts: %#v", posts)
	}
	if absoluteURL("https://example.com/", "posts/hello/") != "https://example.com/posts/hello/" {
		t.Fatal("unexpected absolute url")
	}
	if choose("a", "b") != "a" || choose(" ", "b") != "b" {
		t.Fatal("unexpected choose result")
	}
	_ = cfg
}

func testFeedGraph() (*config.Config, *content.SiteGraph) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	older := now.Add(-24 * time.Hour)

	cfg := &config.Config{
		Title:   "Foundry",
		BaseURL: "https://example.com/",
		Feed: config.FeedConfig{
			RSSTitle:       "Feed Title",
			RSSDescription: "Feed Description",
			RSSLimit:       10,
		},
	}
	graph := content.NewSiteGraph(cfg)
	graph.ByType["post"] = []*content.Document{
		{Title: "Older", URL: "/posts/older/", Summary: "Older summary", Date: &older},
		{Title: "Draft", URL: "/posts/draft-post/", Draft: true, Date: &now},
		{Title: "Newer", URL: "/posts/newer/", Summary: "Newer summary", Date: &now, UpdatedAt: &now},
	}
	graph.Documents = []*content.Document{
		graph.ByType["post"][0],
		graph.ByType["post"][1],
		graph.ByType["post"][2],
	}
	return cfg, graph
}
