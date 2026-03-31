package config

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/sphireinc/foundry/internal/safepath"
)

var adminPathSegmentRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
	require("backup.dir", cfg.Backup.Dir)
	require("content_dir", cfg.ContentDir)
	require("public_dir", cfg.PublicDir)
	require("themes_dir", cfg.ThemesDir)
	require("data_dir", cfg.DataDir)
	require("plugins_dir", cfg.PluginsDir)

	require("content.pages_dir", cfg.Content.PagesDir)
	require("content.posts_dir", cfg.Content.PostsDir)
	require("content.images_dir", cfg.Content.ImagesDir)
	require("content.videos_dir", cfg.Content.VideoDir)
	require("content.audio_dir", cfg.Content.AudioDir)
	require("content.documents_dir", cfg.Content.DocumentsDir)
	require("content.assets_dir", cfg.Content.AssetsDir)
	require("content.uploads_dir", cfg.Content.UploadsDir)
	require("content.default_layout_page", cfg.Content.DefaultLayoutPage)
	require("content.default_layout_post", cfg.Content.DefaultLayoutPost)
	if cfg.Content.MaxVersionsPerFile <= 0 {
		errs = append(errs, fmt.Errorf("content.max_versions_per_file must be greater than zero"))
	}

	require("server.addr", cfg.Server.Addr)
	require("feed.rss_path", cfg.Feed.RSSPath)
	require("feed.sitemap_path", cfg.Feed.SitemapPath)
	require("server.live_reload_mode", cfg.Server.LiveReloadMode)
	if strings.TrimSpace(cfg.Environment) == "" {
		errs = append(errs, fmt.Errorf("environment must not be empty"))
	}

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
	if cfg.Backup.DebounceSeconds <= 0 {
		errs = append(errs, fmt.Errorf("backup.debounce_seconds must be greater than zero"))
	}
	if cfg.Backup.RetentionCount < 0 {
		errs = append(errs, fmt.Errorf("backup.retention_count must not be negative"))
	}
	if cfg.Backup.MinFreeMB < 0 {
		errs = append(errs, fmt.Errorf("backup.min_free_mb must not be negative"))
	}
	if cfg.Backup.HeadroomPercent < 100 {
		errs = append(errs, fmt.Errorf("backup.headroom_percent must be at least 100"))
	}

	if cfg.DefaultLang != "" && strings.Contains(cfg.DefaultLang, "/") {
		errs = append(errs, fmt.Errorf("default_lang must not contain '/'"))
	}
	if _, err := safepath.ValidatePathComponent("theme", cfg.Theme); err != nil {
		errs = append(errs, err)
	}
	adminPath := cfg.AdminPath()
	if !strings.HasPrefix(adminPath, "/") {
		errs = append(errs, fmt.Errorf("admin.path must start with '/'"))
	}
	if adminPath == "/" {
		errs = append(errs, fmt.Errorf("admin.path must not be '/'"))
	}
	if path.Clean(adminPath) != adminPath {
		errs = append(errs, fmt.Errorf("admin.path must be normalized"))
	}
	for _, part := range strings.Split(strings.TrimPrefix(adminPath, "/"), "/") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if _, err := safepath.ValidatePathComponent("admin.path", part); err != nil {
			errs = append(errs, err)
			continue
		}
		if !adminPathSegmentRE.MatchString(part) {
			errs = append(errs, fmt.Errorf("admin.path segments may only contain letters, numbers, '.', '_' or '-'"))
		}
	}
	if _, err := safepath.ValidatePathComponent("admin.theme", cfg.Admin.Theme); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(cfg.Admin.UsersFile) == "" {
		errs = append(errs, fmt.Errorf("admin.users_file must not be empty"))
	}
	if strings.TrimSpace(cfg.Admin.SessionStoreFile) == "" {
		errs = append(errs, fmt.Errorf("admin.session_store_file must not be empty"))
	}
	if strings.TrimSpace(cfg.Admin.LockFile) == "" {
		errs = append(errs, fmt.Errorf("admin.lock_file must not be empty"))
	}
	if cfg.Admin.SessionTTLMinutes <= 0 {
		errs = append(errs, fmt.Errorf("admin.session_ttl_minutes must be greater than zero"))
	}
	if cfg.Admin.PasswordMinLength < 8 {
		errs = append(errs, fmt.Errorf("admin.password_min_length must be at least 8"))
	}
	if cfg.Admin.PasswordResetTTL <= 0 {
		errs = append(errs, fmt.Errorf("admin.password_reset_ttl_minutes must be greater than zero"))
	}
	if strings.TrimSpace(cfg.Admin.TOTPIssuer) == "" {
		errs = append(errs, fmt.Errorf("admin.totp_issuer must not be empty"))
	}
	for _, name := range cfg.Plugins.Enabled {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, err := safepath.ValidatePathComponent("plugin name", name); err != nil {
			errs = append(errs, err)
		}
	}
	if strings.TrimSpace(cfg.Deploy.DefaultTarget) != "" {
		if _, ok := cfg.Deploy.Targets[strings.TrimSpace(cfg.Deploy.DefaultTarget)]; !ok {
			errs = append(errs, fmt.Errorf("deploy.default_target must reference a configured deploy target"))
		}
	}
	for name, target := range cfg.Deploy.Targets {
		if _, err := safepath.ValidatePathComponent("deploy target", name); err != nil {
			errs = append(errs, err)
		}
		if strings.TrimSpace(target.Theme) != "" {
			if _, err := safepath.ValidatePathComponent("deploy target theme", target.Theme); err != nil {
				errs = append(errs, err)
			}
		}
		if strings.TrimSpace(target.LiveReloadMode) != "" {
			switch strings.ToLower(strings.TrimSpace(target.LiveReloadMode)) {
			case "stream", "poll":
			default:
				errs = append(errs, fmt.Errorf("deploy.targets.%s.live_reload_mode must be one of: stream, poll", name))
			}
		}
		if strings.TrimSpace(target.Environment) != "" && strings.Contains(target.Environment, "/") {
			errs = append(errs, fmt.Errorf("deploy.targets.%s.environment must not contain '/'", name))
		}
	}

	return errs
}
