package httpadmin

import (
	"net/http"

	"github.com/sphireinc/foundry/internal/admin/service"
)

func registerStatusRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern: r.routePath("/api/status"),
			handler: http.HandlerFunc(r.handleStatus),
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

var _ service.StatusProvider
