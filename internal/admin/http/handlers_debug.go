package httpadmin

import (
	"net/http"
	"net/http/pprof"
	"strings"
)

// registerDebugRoutes returns optional runtime-debug and pprof routes when
// admin.debug.pprof is enabled.
func registerDebugRoutes(r *Router) []routeDef {
	if r == nil || r.cfg == nil || !r.cfg.Admin.Debug.Pprof {
		return nil
	}
	return []routeDef{
		{
			pattern:    r.routePath("/api/debug/runtime"),
			handler:    http.HandlerFunc(r.handleRuntimeStatus),
			capability: "debug.read",
		},
		{
			pattern:    r.routePath("/debug/pprof/"),
			handler:    http.HandlerFunc(r.handlePprofIndex),
			capability: "debug.read",
		},
		{
			pattern:    r.routePath("/debug/pprof/cmdline"),
			handler:    http.HandlerFunc(r.handlePprofCmdline),
			capability: "debug.read",
		},
		{
			pattern:    r.routePath("/debug/pprof/profile"),
			handler:    http.HandlerFunc(r.handlePprofProfile),
			capability: "debug.read",
		},
		{
			pattern:    r.routePath("/debug/pprof/symbol"),
			handler:    http.HandlerFunc(r.handlePprofSymbol),
			capability: "debug.read",
		},
		{
			pattern:    r.routePath("/debug/pprof/trace"),
			handler:    http.HandlerFunc(r.handlePprofTrace),
			capability: "debug.read",
		},
	}
}

func (r *Router) handleRuntimeStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := r.service.GetRuntimeStatus(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handlePprofIndex(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(req.URL.Path, r.routePath("/debug/pprof/")) {
		http.NotFound(w, req)
		return
	}
	pprof.Index(w, req)
}

func (r *Router) handlePprofCmdline(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pprof.Cmdline(w, req)
}

func (r *Router) handlePprofProfile(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pprof.Profile(w, req)
}

func (r *Router) handlePprofSymbol(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pprof.Symbol(w, req)
}

func (r *Router) handlePprofTrace(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pprof.Trace(w, req)
}
