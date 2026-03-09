package config

import (
	"fmt"
	"strings"
)

func Validate(cfg *Config) []error {
	if cfg == nil {
		return []error{fmt.Errorf("config is nil")}
	}

	var errs []error

	require := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s must not be empty", name))
		}
	}

	require("theme", cfg.Theme)
	require("default_lang", cfg.DefaultLang)
	require("content_dir", cfg.ContentDir)
	require("public_dir", cfg.PublicDir)
	require("themes_dir", cfg.ThemesDir)
	require("data_dir", cfg.DataDir)
	require("plugins_dir", cfg.PluginsDir)

	require("content.pages_dir", cfg.Content.PagesDir)
	require("content.posts_dir", cfg.Content.PostsDir)
	require("content.images_dir", cfg.Content.ImagesDir)
	require("content.assets_dir", cfg.Content.AssetsDir)
	require("content.uploads_dir", cfg.Content.UploadsDir)
	require("content.default_layout_page", cfg.Content.DefaultLayoutPage)
	require("content.default_layout_post", cfg.Content.DefaultLayoutPost)

	require("server.addr", cfg.Server.Addr)
	require("feed.rss_path", cfg.Feed.RSSPath)
	require("feed.sitemap_path", cfg.Feed.SitemapPath)

	if cfg.Feed.RSSPath != "" && !strings.HasPrefix(cfg.Feed.RSSPath, "/") {
		errs = append(errs, fmt.Errorf("feed.rss_path must start with '/'"))
	}
	if cfg.Feed.SitemapPath != "" && !strings.HasPrefix(cfg.Feed.SitemapPath, "/") {
		errs = append(errs, fmt.Errorf("feed.sitemap_path must start with '/'"))
	}
	if cfg.Feed.RSSPath != "" && cfg.Feed.RSSPath == cfg.Feed.SitemapPath {
		errs = append(errs, fmt.Errorf("feed.rss_path and feed.sitemap_path must not be the same"))
	}

	if cfg.DefaultLang != "" && strings.Contains(cfg.DefaultLang, "/") {
		errs = append(errs, fmt.Errorf("default_lang must not contain '/'"))
	}

	return errs
}
