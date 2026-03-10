package httpadmin

import (
	"net/http"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/service"
	adminui "github.com/sphireinc/foundry/internal/admin/ui"
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

func NewHooks(cfg *config.Config, base server.Hooks, opts ...service.Option) server.Hooks {
	if cfg == nil || !cfg.Admin.Enabled {
		if base == nil {
			return hookBase{}
		}
		return base
	}

	svc := service.New(cfg, opts...)
	router := New(cfg, svc)
	return WrapHooks(base, router)
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
	_, _ = w.Write([]byte(adminui.IndexHTML(r.cfg.Title)))
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
