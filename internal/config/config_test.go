package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if cfg.Name == "" || cfg.Theme == "" || cfg.Server.Addr == "" {
		t.Fatalf("expected defaults to be applied: %#v", cfg)
	}
	if cfg.Server.LiveReloadMode != "stream" {
		t.Fatalf("expected live reload mode default to be stream, got %q", cfg.Server.LiveReloadMode)
	}
	if cfg.Admin.LocalOnly != true {
		t.Fatalf("expected admin local only default to be true")
	}
	if cfg.Admin.Theme != "default" {
		t.Fatalf("expected admin theme default to be default, got %q", cfg.Admin.Theme)
	}
	if cfg.Admin.UsersFile != filepath.Join("content", "config", "admin-users.yaml") {
		t.Fatalf("expected default admin users file, got %q", cfg.Admin.UsersFile)
	}
	if cfg.Admin.SessionStoreFile != filepath.Join("data", "admin", "sessions.yaml") {
		t.Fatalf("expected default admin session store file, got %q", cfg.Admin.SessionStoreFile)
	}
	if cfg.Admin.LockFile != filepath.Join("data", "admin", "locks.yaml") {
		t.Fatalf("expected default admin lock file, got %q", cfg.Admin.LockFile)
	}
	if cfg.AdminPath() != "/__admin" {
		t.Fatalf("expected default admin path, got %q", cfg.AdminPath())
	}
	if cfg.Admin.SessionTTLMinutes != 30 {
		t.Fatalf("expected default admin session ttl, got %d", cfg.Admin.SessionTTLMinutes)
	}
	if cfg.Admin.Debug.Pprof {
		t.Fatal("expected admin pprof debug to default to disabled")
	}
	if cfg.Content.MaxVersionsPerFile != 10 {
		t.Fatalf("expected default content max versions, got %d", cfg.Content.MaxVersionsPerFile)
	}
	if cfg.Feed.RSSPath == "" || cfg.Feed.SitemapPath == "" {
		t.Fatalf("expected feed defaults to be set")
	}
}

func TestLoadValidateAndEditYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "site.yaml")
	body := []byte("theme: default\ndefault_lang: en\ncontent_dir: content\npublic_dir: public\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\nserver:\n  addr: :8080\n  live_reload_mode: poll\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\nplugins:\n  enabled:\n    - toc\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if errs := Validate(cfg); len(errs) != 0 {
		t.Fatalf("expected valid config, got %v", errs)
	}

	if err := UpsertTopLevelScalar(path, "theme", "custom"); err != nil {
		t.Fatalf("upsert top level scalar: %v", err)
	}
	if err := EnsureStringListValue(path, []string{"plugins", "enabled"}, "search"); err != nil {
		t.Fatalf("ensure string list value: %v", err)
	}
	if err := RemoveStringListValue(path, []string{"plugins", "enabled"}, "toc"); err != nil {
		t.Fatalf("remove string list value: %v", err)
	}

	doc, err := LoadYAMLDocument(path)
	if err != nil {
		t.Fatalf("load yaml doc: %v", err)
	}
	if len(doc.Content) == 0 {
		t.Fatal("expected yaml content")
	}
}

func TestValidateAndSequenceHelpersErrors(t *testing.T) {
	cfg := &Config{
		Theme:   "..",
		Admin:   AdminConfig{Enabled: true, Theme: "../escape"},
		Plugins: PluginConfig{Enabled: []string{"../escape"}},
		Server:  ServerConfig{LiveReloadMode: "invalid"},
		Feed: FeedConfig{
			RSSPath:     "rss.xml",
			SitemapPath: "rss.xml",
		},
		DefaultLang: "en/us",
	}
	if errs := Validate(cfg); len(errs) == 0 {
		t.Fatal("expected validation errors")
	}

	if _, err := ensureSequenceAtPath(nil, []string{"plugins"}); err == nil {
		t.Fatal("expected root nil error")
	}
	if _, err := findSequenceAtPath(nil, []string{"plugins"}); err == nil {
		t.Fatal("expected root nil error")
	}
}

func TestAdminPathNormalizationAndValidation(t *testing.T) {
	cfg := &Config{
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  "content",
		PublicDir:   "public",
		ThemesDir:   "themes",
		DataDir:     "data",
		PluginsDir:  "plugins",
		Admin: AdminConfig{
			Path:              "cms-admin/",
			Theme:             "default",
			UsersFile:         filepath.Join("content", "config", "admin-users.yaml"),
			SessionTTLMinutes: 30,
		},
		Server: ServerConfig{
			Addr:           ":8080",
			LiveReloadMode: "stream",
		},
		Content: ContentConfig{
			PagesDir:           "pages",
			PostsDir:           "posts",
			ImagesDir:          "images",
			AssetsDir:          "assets",
			UploadsDir:         "uploads",
			MaxVersionsPerFile: 10,
			DefaultLayoutPage:  "page",
			DefaultLayoutPost:  "post",
		},
		Feed: FeedConfig{
			RSSPath:     "/rss.xml",
			SitemapPath: "/sitemap.xml",
		},
	}
	cfg.ApplyDefaults()
	if cfg.AdminPath() != "/cms-admin" {
		t.Fatalf("expected normalized admin path, got %q", cfg.AdminPath())
	}
	if errs := Validate(cfg); len(errs) != 0 {
		t.Fatalf("expected custom admin path to validate, got %v", errs)
	}

	cfg.Admin.Path = "/bad path"
	if errs := Validate(cfg); len(errs) == 0 {
		t.Fatal("expected invalid admin path errors")
	}
}

func TestLoadWithOptionsAppliesEnvironmentOverlayAndDeployTarget(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, "site.yaml")
	if err := os.WriteFile(basePath, []byte("theme: default\nbase_url: https://example.com\npublic_dir: public\ncontent_dir: content\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\ndefault_lang: en\ndeploy:\n  default_target: production\n  targets:\n    production:\n      base_url: https://prod.example.com\n      public_dir: public-prod\n      include_drafts: false\nserver:\n  addr: :8080\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\n"), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "site.preview.yaml"), []byte("environment: preview\nbuild:\n  include_drafts: true\n"), 0o644); err != nil {
		t.Fatalf("write overlay config: %v", err)
	}

	cfg, err := LoadWithOptions(basePath, LoadOptions{Environment: "preview", Target: "production"})
	if err != nil {
		t.Fatalf("load with options: %v", err)
	}
	if cfg.Environment != "preview" {
		t.Fatalf("expected environment overlay, got %q", cfg.Environment)
	}
	if cfg.BaseURL != "https://prod.example.com" {
		t.Fatalf("expected deploy target base url override, got %q", cfg.BaseURL)
	}
	if cfg.PublicDir != "public-prod" {
		t.Fatalf("expected deploy target public dir override, got %q", cfg.PublicDir)
	}
	if cfg.Build.IncludeDrafts {
		t.Fatal("expected deploy target include_drafts=false to win")
	}
}

func TestLoadWithOptionsAppliesDefaultDeployTargetWhenNoTargetFlagProvided(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, "site.yaml")
	body := "theme: default\nbase_url: https://example.com\npublic_dir: public\ncontent_dir: content\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\ndefault_lang: en\ndeploy:\n  default_target: production\n  targets:\n    production:\n      base_url: https://prod.example.com\n      public_dir: public-prod\nserver:\n  addr: :8080\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\n"
	if err := os.WriteFile(basePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	cfg, err := LoadWithOptions(basePath, LoadOptions{})
	if err != nil {
		t.Fatalf("load with default target: %v", err)
	}
	if cfg.BaseURL != "https://prod.example.com" || cfg.PublicDir != "public-prod" {
		t.Fatalf("expected default target overrides, got %#v", cfg)
	}
}

func TestUnmarshalYAMLPreservesExplicitAdminLocalOnlyFalse(t *testing.T) {
	cfg := &Config{}
	body := []byte("theme: default\ncontent_dir: content\npublic_dir: public\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\ndefault_lang: en\nadmin:\n  local_only: false\nserver:\n  addr: :8080\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\n")
	if err := UnmarshalYAML(body, cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Admin.LocalOnly {
		t.Fatal("expected explicit admin.local_only=false to be preserved")
	}
}
