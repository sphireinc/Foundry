package httpadmin

import (
	"net/http"
	"strings"

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
	public  bool
}

type Registrar func(*Router) []routeDef

type Router struct {
	cfg        *config.Config
	service    *service.Service
	auth       *adminauth.Middleware
	ui         *adminui.Manager
	registrars []Registrar
}

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

	mux.Handle("/__admin", http.HandlerFunc(r.handleIndex))
	mux.Handle("/__admin/", http.HandlerFunc(r.handleIndex))
	mux.Handle("/__admin/theme/", http.StripPrefix("/__admin/theme/", r.ui.AssetHandler()))

	for _, reg := range r.registrars {
		for _, rd := range reg(r) {
			if rd.public {
				mux.Handle(rd.pattern, rd.handler)
				continue
			}
			mux.Handle(rd.pattern, r.auth.Wrap(rd.handler))
		}
	}
}

func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, "/__admin") || strings.HasPrefix(req.URL.Path, "/__admin/api/") || strings.HasPrefix(req.URL.Path, "/__admin/theme/") {
		http.NotFound(w, req)
		return
	}

	body, err := r.ui.RenderIndex()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
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
