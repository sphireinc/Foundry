package httpadmin

import (
	"encoding/json"
	"net/http"
	"strings"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

func registerAuthRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern: r.routePath("/api/login"),
			handler: http.HandlerFunc(r.handleLogin),
			public:  true,
		},
		{
			pattern: r.routePath("/api/logout"),
			handler: http.HandlerFunc(r.handleLogout),
			public:  true,
		},
		{
			pattern: r.routePath("/api/session"),
			handler: http.HandlerFunc(r.handleSession),
		},
	}
}

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.LoginRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	identity, err := r.auth.Login(w, req, strings.TrimSpace(body.Username), body.Password)
	if err != nil {
		r.logAudit(strings.TrimSpace(body.Username), "login", "failure", strings.TrimSpace(body.Username), map[string]string{"error": err.Error()})
		writeJSONError(w, http.StatusForbidden, err)
		return
	}
	r.logAudit(firstNonEmpty(identity.Name, identity.Username), "login", "success", identity.Username, nil)

	writeJSON(w, http.StatusOK, admintypes.SessionResponse{
		Authenticated: true,
		Username:      identity.Username,
		Name:          identity.Name,
		Email:         identity.Email,
		Role:          identity.Role,
		TTLSeconds:    int(r.auth.SessionTTL().Seconds()),
	})
}

func (r *Router) handleLogout(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.auth.Logout(w, req); err != nil {
		r.logAuditRequest(req, "logout", "failure", "", map[string]string{"error": err.Error()})
		writeJSONError(w, http.StatusForbidden, err)
		return
	}
	r.logAuditRequest(req, "logout", "success", "", nil)

	writeJSON(w, http.StatusOK, admintypes.SessionResponse{})
}

func (r *Router) handleSession(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity, ok := adminauth.IdentityFromContext(req.Context())
	if !ok {
		writeJSONErrorMessage(w, http.StatusForbidden, "admin login is required")
		return
	}

	writeJSON(w, http.StatusOK, admintypes.SessionResponse{
		Authenticated: true,
		Username:      identity.Username,
		Name:          identity.Name,
		Email:         identity.Email,
		Role:          identity.Role,
		TTLSeconds:    int(r.auth.SessionTTL().Seconds()),
	})
}
