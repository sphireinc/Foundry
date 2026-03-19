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
	if cfg.Admin.SessionTTLMinutes != 30 {
		t.Fatalf("expected default admin session ttl, got %d", cfg.Admin.SessionTTLMinutes)
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
