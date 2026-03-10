package config

type Config struct {
	Name        string                `yaml:"name"`
	Title       string                `yaml:"title"`
	BaseURL     string                `yaml:"base_url"`
	Theme       string                `yaml:"theme"`
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
	Params      map[string]any        `yaml:"params"`
	Menus       map[string][]MenuItem `yaml:"menus"`
}

type AdminConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Addr      string `yaml:"addr"`
	LocalOnly bool   `yaml:"local_only"`
}

type ServerConfig struct {
	Addr            string `yaml:"addr"`
	LiveReload      bool   `yaml:"live_reload"`
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
	AssetsDir            string `yaml:"assets_dir"`
	UploadsDir           string `yaml:"uploads_dir"`
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
	Enabled       bool `yaml:"enabled"`
	AllowAnything bool `yaml:"allow_anything"`
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

type MenuItem struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func (c *Config) ApplyDefaults() {
	if c.Name == "" {
		c.Name = "foundry"
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
	if !c.Admin.LocalOnly {
		c.Admin.LocalOnly = true
	}
	if c.DefaultLang == "" {
		c.DefaultLang = "en"
	}
	if c.ContentDir == "" {
		c.ContentDir = "content"
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
	if c.PluginsDir == "" {
		c.PluginsDir = "plugins"
	}
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
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
	if c.Content.AssetsDir == "" {
		c.Content.AssetsDir = "assets"
	}
	if c.Content.UploadsDir == "" {
		c.Content.UploadsDir = "uploads"
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
