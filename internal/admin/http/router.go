package httpadmin

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/service"
	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/safepath"
	"github.com/sphireinc/foundry/internal/server"
)

// routeDef describes one admin HTTP route and the auth/capability wrapper it
// should receive when mounted.
type routeDef struct {
	pattern    string
	handler    http.Handler
	public     bool
	capability string
}

// Registrar returns a cohesive group of admin routes, such as auth, documents,
// management, or debug.
type Registrar func(*Router) []routeDef

// Router owns the authenticated admin HTTP surface and the theme-backed admin
// shell.
type Router struct {
	cfg        *config.Config
	service    *service.Service
	auth       *adminauth.Middleware
	ui         *adminui.Manager
	registrars []Registrar
}

// New constructs the admin router and registers the built-in route groups.
func New(cfg *config.Config, svc *service.Service) *Router {
	r := &Router{
		cfg:        cfg,
		service:    svc,
		auth:       adminauth.New(cfg),
		ui:         adminui.NewManager(cfg),
		registrars: make([]Registrar, 0),
	}

	r.RegisterRegistrar(registerAuthRoutes)
	r.RegisterRegistrar(registerStatusRoutes)
	r.RegisterRegistrar(registerDocumentRoutes)
	r.RegisterRegistrar(registerManagementRoutes)
	r.RegisterRegistrar(registerDebugRoutes)
	return r
}

// NewHooks composes the admin router into the preview-server hook chain when
// admin is enabled.
func NewHooks(cfg *config.Config, base server.Hooks, opts ...service.Option) server.Hooks {
	if cfg == nil || !cfg.Admin.Enabled {
		if base == nil {
			return hookBase{}
		}
		return base
	}

	if metadata := pluginMetadataProvider(base); metadata != nil {
		opts = append(opts, service.WithPluginMetadata(metadata))
	}
	svc := service.New(cfg, opts...)
	router := New(cfg, svc)
	return WrapHooks(base, router)
}

func pluginMetadataProvider(h any) func() map[string]plugins.Metadata {
	for h != nil {
		if pm, ok := h.(interface {
			Metadata() map[string]plugins.Metadata
		}); ok {
			return pm.Metadata
		}
		unwrap, ok := h.(interface {
			UnwrapHooks() any
		})
		if !ok {
			return nil
		}
		h = unwrap.UnwrapHooks()
	}
	return nil
}

// RegisterRegistrar appends an admin route group to the router.
func (r *Router) RegisterRegistrar(reg Registrar) {
	if reg == nil {
		return
	}
	r.registrars = append(r.registrars, reg)
}

// RegisterRoutes mounts the admin shell, admin theme assets, plugin extension
// assets, and all registered admin route groups.
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	if r == nil || mux == nil || r.cfg == nil || !r.cfg.Admin.Enabled {
		return
	}

	mux.Handle(r.adminBasePath(), http.HandlerFunc(r.handleIndex))
	mux.Handle(r.adminBasePath()+"/", http.HandlerFunc(r.handleIndex))
	mux.Handle(r.themeBasePath()+"/", http.StripPrefix(r.themeBasePath()+"/", r.ui.AssetHandler()))
	mux.Handle(r.extensionAssetBasePath()+"/", r.auth.Wrap(http.HandlerFunc(r.handlePluginExtensionAsset)))

	for _, reg := range r.registrars {
		for _, rd := range reg(r) {
			if rd.public {
				mux.Handle(rd.pattern, rd.handler)
				continue
			}
			if strings.TrimSpace(rd.capability) != "" {
				mux.Handle(rd.pattern, r.auth.WrapCapability(rd.handler, rd.capability))
				continue
			}
			mux.Handle(rd.pattern, r.auth.Wrap(rd.handler))
		}
	}
}

// handleIndex serves the admin shell for non-API admin routes.
func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, r.adminBasePath()) ||
		strings.HasPrefix(req.URL.Path, r.apiBasePath()+"/") ||
		strings.HasPrefix(req.URL.Path, r.themeBasePath()+"/") ||
		strings.HasPrefix(req.URL.Path, r.routePath("/debug/pprof/")) {
		http.NotFound(w, req)
		return
	}

	body, err := r.ui.RenderIndex()
	if err != nil {
		http.Error(w, "admin UI render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'self'; img-src 'self' data: blob:; media-src 'self' blob:; frame-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'")
	_, _ = w.Write(body)
}

func (r *Router) adminBasePath() string {
	if r == nil || r.cfg == nil {
		return "/__admin"
	}
	return r.cfg.AdminPath()
}

func (r *Router) apiBasePath() string {
	return r.adminBasePath() + "/api"
}

func (r *Router) themeBasePath() string {
	return r.adminBasePath() + "/theme"
}

func (r *Router) extensionAssetBasePath() string {
	return r.adminBasePath() + "/extensions"
}

func (r *Router) routePath(suffix string) string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" || suffix == "/" {
		return r.adminBasePath()
	}
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	return r.adminBasePath() + suffix
}

// handlePluginExtensionAsset serves plugin-declared admin JS/CSS bundles after
// verifying the requested asset is part of the plugin's published admin
// contract.
func (r *Router) handlePluginExtensionAsset(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(req.URL.Path, r.extensionAssetBasePath()+"/")
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, req)
		return
	}

	pluginName, err := safepath.ValidatePathComponent("plugin name", parts[0])
	if err != nil {
		http.NotFound(w, req)
		return
	}
	assetPath, err := plugins.NormalizeAdminAssetPath(parts[1])
	if err != nil {
		http.NotFound(w, req)
		return
	}
	if r.service == nil || !r.service.AllowsAdminAsset(pluginName, assetPath) {
		http.NotFound(w, req)
		return
	}

	root := filepath.Join(r.cfg.PluginsDir, pluginName)
	target, err := safepath.ResolveRelativeUnderRoot(root, assetPath)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, req, target)
}

// WrapHooks combines an existing preview-server hook set with the admin router.
func WrapHooks(base server.Hooks, admin *Router) server.Hooks {
	if base == nil {
		base = hookBase{}
	}
	if admin == nil {
		return base
	}

	return hookSet{
		base:  base,
		admin: admin,
	}
}

type hookSet struct {
	base  server.Hooks
	admin *Router
}

func (h hookSet) RegisterRoutes(mux *http.ServeMux) {
	h.base.RegisterRoutes(mux)
	h.admin.RegisterRoutes(mux)
}

func (h hookSet) OnServerStarted(addr string) error {
	return h.base.OnServerStarted(addr)
}

func (h hookSet) OnRoutesAssigned(graph *content.SiteGraph) error {
	return h.base.OnRoutesAssigned(graph)
}

func (h hookSet) OnAssetsBuilding(cfg *config.Config) error {
	return h.base.OnAssetsBuilding(cfg)
}

type hookBase struct{}

func (hookBase) RegisterRoutes(_ *http.ServeMux)             {}
func (hookBase) OnServerStarted(_ string) error              { return nil }
func (hookBase) OnRoutesAssigned(_ *content.SiteGraph) error { return nil }
func (hookBase) OnAssetsBuilding(_ *config.Config) error     { return nil }
