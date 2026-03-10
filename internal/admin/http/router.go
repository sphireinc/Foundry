package httpadmin

import (
	"fmt"
	"html"
	"net/http"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/service"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/server"
)

type routeDef struct {
	pattern string
	handler http.Handler
}

type Registrar func(*Router) []routeDef

type Router struct {
	cfg        *config.Config
	service    *service.Service
	auth       *adminauth.Middleware
	registrars []Registrar
}

func New(cfg *config.Config, svc *service.Service) *Router {
	r := &Router{
		cfg:        cfg,
		service:    svc,
		auth:       adminauth.New(cfg),
		registrars: make([]Registrar, 0),
	}

	r.RegisterRegistrar(registerStatusRoutes)
	r.RegisterRegistrar(registerDocumentRoutes)
	return r
}

func (r *Router) RegisterRegistrar(reg Registrar) {
	if reg == nil {
		return
	}
	r.registrars = append(r.registrars, reg)
}

func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	if r == nil || mux == nil || r.cfg == nil || !r.cfg.Admin.Enabled {
		return
	}

	mux.Handle("/__admin", r.auth.Wrap(http.HandlerFunc(r.handleIndex)))
	mux.Handle("/__admin/", r.auth.Wrap(http.HandlerFunc(r.handleIndex)))

	for _, reg := range r.registrars {
		for _, rd := range reg(r) {
			mux.Handle(rd.pattern, r.auth.Wrap(rd.handler))
		}
	}
}

func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>%s Admin</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    body { font-family: system-ui, sans-serif; max-width: 860px; margin: 40px auto; padding: 0 20px; line-height: 1.5; }
    h1 { margin-bottom: 0.25rem; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 6px; }
    ul { padding-left: 20px; }
    .muted { color: #666; }
  </style>
</head>
<body>
  <h1>%s Admin</h1>
  <p class="muted">Filesystem-first admin boundary for Foundry.</p>

  <h2>Available endpoints</h2>
  <ul>
    <li><code>GET /__admin/api/status</code></li>
    <li><code>GET /__admin/api/documents</code></li>
    <li><code>GET /__admin/api/document?id=&lt;document-id-or-path&gt;</code></li>
    <li><code>POST /__admin/api/documents/save</code></li>
    <li><code>POST /__admin/api/documents/preview</code></li>
  </ul>
</body>
</html>`, html.EscapeString(r.cfg.Title), html.EscapeString(r.cfg.Title))
}

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
