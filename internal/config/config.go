package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type Config struct {
	Name        string                `yaml:"name"`
	Title       string                `yaml:"title"`
	BaseURL     string                `yaml:"base_url"`
	Theme       string                `yaml:"theme"`
	Environment string                `yaml:"environment"`
	Admin       AdminConfig           `yaml:"admin"`
	DefaultLang string                `yaml:"default_lang"`
	ContentDir  string                `yaml:"content_dir"`
	PublicDir   string                `yaml:"public_dir"`
	ThemesDir   string                `yaml:"themes_dir"`
	DataDir     string                `yaml:"data_dir"`
	PluginsDir  string                `yaml:"plugins_dir"`
	Permalinks  map[string]string     `yaml:"permalinks"`
	Server      ServerConfig          `yaml:"server"`
	Build       BuildConfig           `yaml:"build"`
	Content     ContentConfig         `yaml:"content"`
	Taxonomies  TaxonomyConfig        `yaml:"taxonomies"`
	Plugins     PluginConfig          `yaml:"plugins"`
	Fields      FieldsConfig          `yaml:"fields"`
	SEO         SEOConfig             `yaml:"seo"`
	Cache       CacheConfig           `yaml:"cache"`
	Security    SecurityConfig        `yaml:"security"`
	Feed        FeedConfig            `yaml:"feed"`
	Deploy      DeployConfig          `yaml:"deploy"`
	Params      map[string]any        `yaml:"params"`
	Menus       map[string][]MenuItem `yaml:"menus"`
}

type AdminConfig struct {
	Enabled           bool             `yaml:"enabled"`
	Addr              string           `yaml:"addr"`
	Path              string           `yaml:"path"`
	Debug             AdminDebugConfig `yaml:"debug"`
	LocalOnly         bool             `yaml:"local_only"`
	AccessToken       string           `yaml:"access_token"`
	Theme             string           `yaml:"theme"`
	UsersFile         string           `yaml:"users_file"`
	SessionStoreFile  string           `yaml:"session_store_file"`
	LockFile          string           `yaml:"lock_file"`
	SessionTTLMinutes int              `yaml:"session_ttl_minutes"`
	PasswordMinLength int              `yaml:"password_min_length"`
	PasswordResetTTL  int              `yaml:"password_reset_ttl_minutes"`
	TOTPIssuer        string           `yaml:"totp_issuer"`
	localOnlySet      bool             `yaml:"-"`
}

type AdminDebugConfig struct {
	Pprof bool `yaml:"pprof"`
}

type ServerConfig struct {
	Addr            string `yaml:"addr"`
	LiveReload      bool   `yaml:"live_reload"`
	LiveReloadMode  string `yaml:"live_reload_mode"`
	AutoOpenBrowser bool   `yaml:"auto_open_browser"`
	DebugRoutes     bool   `yaml:"debug_routes"`
}

type BuildConfig struct {
	CleanPublicDir bool `yaml:"clean_public_dir"`
	IncludeDrafts  bool `yaml:"include_drafts"`
	MinifyHTML     bool `yaml:"minify_html"`
	CopyAssets     bool `yaml:"copy_assets"`
	CopyImages     bool `yaml:"copy_images"`
	CopyUploads    bool `yaml:"copy_uploads"`
}

type ContentConfig struct {
	PagesDir             string `yaml:"pages_dir"`
	PostsDir             string `yaml:"posts_dir"`
	ImagesDir            string `yaml:"images_dir"`
	VideoDir             string `yaml:"videos_dir"`
	AudioDir             string `yaml:"audio_dir"`
	DocumentsDir         string `yaml:"documents_dir"`
	AssetsDir            string `yaml:"assets_dir"`
	UploadsDir           string `yaml:"uploads_dir"`
	MaxVersionsPerFile   int    `yaml:"max_versions_per_file"`
	DefaultLayoutPage    string `yaml:"default_layout_page"`
	DefaultLayoutPost    string `yaml:"default_layout_post"`
	DefaultPageSlugIndex string `yaml:"default_page_slug_index"`
}

type TaxonomyConfig struct {
	Enabled     bool                          `yaml:"enabled"`
	DefaultSet  []string                      `yaml:"default_set"`
	Definitions map[string]TaxonomyDefinition `yaml:"definitions"`
}

type TaxonomyDefinition struct {
	Title         string            `yaml:"title"`
	Labels        map[string]string `yaml:"labels"`
	ArchiveLayout string            `yaml:"archive_layout"`
	TermLayout    string            `yaml:"term_layout"`
	Order         string            `yaml:"order"`
}

type PluginConfig struct {
	Enabled []string `yaml:"enabled"`
}

type FieldsConfig struct {
	Enabled       bool                      `yaml:"enabled"`
	AllowAnything bool                      `yaml:"allow_anything"`
	Schemas       map[string]FieldSchemaSet `yaml:"schemas"`
}

type FieldSchemaSet struct {
	Fields []FieldDefinition `yaml:"fields"`
}

type FieldDefinition struct {
	Name        string            `yaml:"name"`
	Label       string            `yaml:"label,omitempty"`
	Type        string            `yaml:"type"`
	Required    bool              `yaml:"required,omitempty"`
	Default     any               `yaml:"default,omitempty"`
	Enum        []string          `yaml:"enum,omitempty"`
	Fields      []FieldDefinition `yaml:"fields,omitempty"`
	Item        *FieldDefinition  `yaml:"item,omitempty"`
	Help        string            `yaml:"help,omitempty"`
	Placeholder string            `yaml:"placeholder,omitempty"`
}

type SEOConfig struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultTitleSep string `yaml:"default_title_sep"`
}

type CacheConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SecurityConfig struct {
	AllowUnsafeHTML bool `yaml:"allow_unsafe_html"`
}

type FeedConfig struct {
	RSSPath        string `yaml:"rss_path"`
	SitemapPath    string `yaml:"sitemap_path"`
	RSSLimit       int    `yaml:"rss_limit"`
	RSSTitle       string `yaml:"rss_title"`
	RSSDescription string `yaml:"rss_description"`
}

type DeployConfig struct {
	DefaultTarget string                  `yaml:"default_target"`
	Targets       map[string]DeployTarget `yaml:"targets"`
}

type DeployTarget struct {
	BaseURL        string `yaml:"base_url"`
	PublicDir      string `yaml:"public_dir"`
	Theme          string `yaml:"theme"`
	IncludeDrafts  *bool  `yaml:"include_drafts"`
	Environment    string `yaml:"environment"`
	Preview        *bool  `yaml:"preview"`
	LiveReloadMode string `yaml:"live_reload_mode"`
}

type MenuItem struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func (c *Config) ApplyDefaults() {
	if c.Name == "" {
		c.Name = "foundry"
	}
	if strings.TrimSpace(c.Environment) == "" {
		c.Environment = "default"
	}
	if c.Title == "" {
		c.Title = "Foundry CMS"
	}
	if c.Theme == "" {
		c.Theme = "default"
	}
	if c.Admin.Addr == "" {
		c.Admin.Addr = ""
	}
	c.Admin.Path = normalizeAdminPath(c.Admin.Path)
	if strings.TrimSpace(c.Admin.Theme) == "" {
		c.Admin.Theme = "default"
	}
	if c.Admin.SessionTTLMinutes <= 0 {
		c.Admin.SessionTTLMinutes = 30
	}
	if !c.Admin.localOnlySet && !c.Admin.LocalOnly {
		c.Admin.LocalOnly = true
	}
	if c.DefaultLang == "" {
		c.DefaultLang = "en"
	}
	if c.ContentDir == "" {
		c.ContentDir = "content"
	}
	if strings.TrimSpace(c.Admin.UsersFile) == "" {
		c.Admin.UsersFile = filepath.Join(c.ContentDir, "config", "admin-users.yaml")
	}
	if c.Admin.PasswordMinLength <= 0 {
		c.Admin.PasswordMinLength = 12
	}
	if c.Admin.PasswordResetTTL <= 0 {
		c.Admin.PasswordResetTTL = 30
	}
	if strings.TrimSpace(c.Admin.TOTPIssuer) == "" {
		c.Admin.TOTPIssuer = "Foundry"
	}
	if c.PublicDir == "" {
		c.PublicDir = "public"
	}
	if c.ThemesDir == "" {
		c.ThemesDir = "themes"
	}
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if strings.TrimSpace(c.Admin.SessionStoreFile) == "" {
		c.Admin.SessionStoreFile = filepath.Join(c.DataDir, "admin", "sessions.yaml")
	}
	if strings.TrimSpace(c.Admin.LockFile) == "" {
		c.Admin.LockFile = filepath.Join(c.DataDir, "admin", "locks.yaml")
	}
	if c.PluginsDir == "" {
		c.PluginsDir = "plugins"
	}
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if strings.TrimSpace(c.Server.LiveReloadMode) == "" {
		c.Server.LiveReloadMode = "stream"
	} else {
		c.Server.LiveReloadMode = strings.ToLower(strings.TrimSpace(c.Server.LiveReloadMode))
	}
	if c.Content.PagesDir == "" {
		c.Content.PagesDir = "pages"
	}
	if c.Content.PostsDir == "" {
		c.Content.PostsDir = "posts"
	}
	if c.Content.ImagesDir == "" {
		c.Content.ImagesDir = "images"
	}
	if c.Content.VideoDir == "" {
		c.Content.VideoDir = "videos"
	}
	if c.Content.AudioDir == "" {
		c.Content.AudioDir = "audio"
	}
	if c.Content.DocumentsDir == "" {
		c.Content.DocumentsDir = "documents"
	}
	if c.Content.AssetsDir == "" {
		c.Content.AssetsDir = "assets"
	}
	if c.Content.UploadsDir == "" {
		c.Content.UploadsDir = "uploads"
	}
	if c.Content.MaxVersionsPerFile <= 0 {
		c.Content.MaxVersionsPerFile = 10
	}
	if c.Content.DefaultLayoutPage == "" {
		c.Content.DefaultLayoutPage = "page"
	}
	if c.Content.DefaultLayoutPost == "" {
		c.Content.DefaultLayoutPost = "post"
	}
	if c.Content.DefaultPageSlugIndex == "" {
		c.Content.DefaultPageSlugIndex = "index"
	}
	if c.Permalinks == nil {
		c.Permalinks = map[string]string{
			"page_default": "/:slug/",
			"page_i18n":    "/:lang/:slug/",
			"post_default": "/posts/:slug/",
			"post_i18n":    "/:lang/posts/:slug/",
		}
	}
	if c.Taxonomies.DefaultSet == nil {
		c.Taxonomies.DefaultSet = []string{"tags", "categories"}
	}
	if c.Taxonomies.Definitions == nil {
		c.Taxonomies.Definitions = map[string]TaxonomyDefinition{}
	}
	if c.Fields.Schemas == nil {
		c.Fields.Schemas = map[string]FieldSchemaSet{}
	}
	if c.Deploy.Targets == nil {
		c.Deploy.Targets = map[string]DeployTarget{}
	}
	if c.Feed.RSSPath == "" {
		c.Feed.RSSPath = "/rss.xml"
	}
	if c.Feed.SitemapPath == "" {
		c.Feed.SitemapPath = "/sitemap.xml"
	}
	if c.Feed.RSSLimit == 0 {
		c.Feed.RSSLimit = 50
	}
	if c.Feed.RSSTitle == "" {
		c.Feed.RSSTitle = c.Title
	}
	if c.Feed.RSSDescription == "" {
		c.Feed.RSSDescription = c.Title
	}
}

func (c *Config) AdminPath() string {
	if c == nil {
		return defaultAdminPath
	}
	return normalizeAdminPath(c.Admin.Path)
}

const defaultAdminPath = "/__admin"

func normalizeAdminPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if value == "" {
		return defaultAdminPath
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = path.Clean(value)
	if value == "." || value == "" {
		return defaultAdminPath
	}
	if value != "/" {
		value = strings.TrimRight(value, "/")
	}
	if value == "/" {
		return defaultAdminPath
	}
	return value
}

func (c *Config) ApplyDeployTarget(name string) error {
	if c == nil || strings.TrimSpace(name) == "" {
		return nil
	}

	target, ok := c.Deploy.Targets[strings.TrimSpace(name)]
	if !ok {
		return fmt.Errorf("unknown deploy target: %s", name)
	}

	if strings.TrimSpace(target.BaseURL) != "" {
		c.BaseURL = strings.TrimSpace(target.BaseURL)
	}
	if strings.TrimSpace(target.PublicDir) != "" {
		c.PublicDir = strings.TrimSpace(target.PublicDir)
	}
	if strings.TrimSpace(target.Theme) != "" {
		c.Theme = strings.TrimSpace(target.Theme)
	}
	if target.IncludeDrafts != nil {
		c.Build.IncludeDrafts = *target.IncludeDrafts
	}
	if strings.TrimSpace(target.Environment) != "" {
		c.Environment = strings.TrimSpace(target.Environment)
	}
	if target.Preview != nil && *target.Preview {
		c.Build.IncludeDrafts = true
	}
	if strings.TrimSpace(target.LiveReloadMode) != "" {
		c.Server.LiveReloadMode = strings.ToLower(strings.TrimSpace(target.LiveReloadMode))
	}
	c.ApplyDefaults()
	return nil
}
