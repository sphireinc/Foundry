package config

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/safepath"
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
	require("server.live_reload_mode", cfg.Server.LiveReloadMode)

	if cfg.Feed.RSSPath != "" && !strings.HasPrefix(cfg.Feed.RSSPath, "/") {
		errs = append(errs, fmt.Errorf("feed.rss_path must start with '/'"))
	}
	if cfg.Feed.SitemapPath != "" && !strings.HasPrefix(cfg.Feed.SitemapPath, "/") {
		errs = append(errs, fmt.Errorf("feed.sitemap_path must start with '/'"))
	}
	if cfg.Feed.RSSPath != "" && cfg.Feed.RSSPath == cfg.Feed.SitemapPath {
		errs = append(errs, fmt.Errorf("feed.rss_path and feed.sitemap_path must not be the same"))
	}
	if cfg.Server.LiveReloadMode != "" {
		switch strings.ToLower(strings.TrimSpace(cfg.Server.LiveReloadMode)) {
		case "stream", "poll":
		default:
			errs = append(errs, fmt.Errorf("server.live_reload_mode must be one of: stream, poll"))
		}
	}

	if cfg.DefaultLang != "" && strings.Contains(cfg.DefaultLang, "/") {
		errs = append(errs, fmt.Errorf("default_lang must not contain '/'"))
	}
	if _, err := safepath.ValidatePathComponent("theme", cfg.Theme); err != nil {
		errs = append(errs, err)
	}
	for _, name := range cfg.Plugins.Enabled {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, err := safepath.ValidatePathComponent("plugin name", name); err != nil {
			errs = append(errs, err)
		}
	}
	if cfg.Admin.Enabled && strings.TrimSpace(cfg.Admin.AccessToken) == "" {
		errs = append(errs, fmt.Errorf("admin.access_token must not be empty when admin is enabled"))
	}

	return errs
}
