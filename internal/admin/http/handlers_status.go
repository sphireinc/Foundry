package httpadmin

import (
	"net/http"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/service"
	"github.com/sphireinc/foundry/internal/admin/types"
)

func registerStatusRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern: r.routePath("/api/status"),
			handler: http.HandlerFunc(r.handleStatus),
		},
		{
			pattern: r.routePath("/api/capabilities"),
			handler: http.HandlerFunc(r.handleCapabilities),
		},
	}
}

func (r *Router) handleStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, err := r.service.GetSystemStatus(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handleCapabilities(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var identity *adminauth.Identity
	if current, ok := adminauth.IdentityFromContext(req.Context()); ok {
		identity = current
	}

	resp := types.CapabilityResponse{
		SDKVersion: "v1",
		Modules: map[string]bool{
			"session":           true,
			"status":            true,
			"documents":         true,
			"media":             true,
			"settings":          true,
			"settings_sections": true,
			"users":             true,
			"themes":            true,
			"plugins":           true,
			"audit":             true,
			"debug":             r != nil && r.cfg != nil && r.cfg.Admin.Debug.Pprof,
			"extensions":        true,
			"sync":              false,
		},
		Features: map[string]bool{
			"history":               true,
			"trash":                 true,
			"diff":                  true,
			"document_locks":        true,
			"workflow":              true,
			"structured_editing":    true,
			"plugin_admin_registry": true,
			"settings_sections":     true,
			"pprof":                 r != nil && r.cfg != nil && r.cfg.Admin.Debug.Pprof,
			"sync":                  false,
			"storage":               false,
		},
	}
	if identity != nil {
		resp.Capabilities = append([]string(nil), identity.Capabilities...)
		resp.Identity = &types.SessionResponse{
			Authenticated: true,
			Username:      identity.Username,
			Name:          identity.Name,
			Email:         identity.Email,
			Role:          identity.Role,
			Capabilities:  append([]string(nil), identity.Capabilities...),
			MFAComplete:   identity.MFAComplete,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

var _ service.StatusProvider
