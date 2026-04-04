package httpadmin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

// registerAuthRoutes returns the authentication and session-management routes
// for the admin API.
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
		{
			pattern:    r.routePath("/api/sessions"),
			handler:    http.HandlerFunc(r.handleSessions),
			capability: "users.manage",
		},
		{
			pattern:    r.routePath("/api/sessions/revoke"),
			handler:    http.HandlerFunc(r.handleSessionRevoke),
			capability: "users.manage",
		},
		{
			pattern:    r.routePath("/api/password-reset/start"),
			handler:    http.HandlerFunc(r.handlePasswordResetStart),
			capability: "dashboard.read",
		},
		{
			pattern: r.routePath("/api/password-reset/complete"),
			handler: http.HandlerFunc(r.handlePasswordResetComplete),
			public:  true,
		},
		{
			pattern:    r.routePath("/api/totp/setup"),
			handler:    http.HandlerFunc(r.handleTOTPSetup),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/totp/enable"),
			handler:    http.HandlerFunc(r.handleTOTPEnable),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/totp/disable"),
			handler:    http.HandlerFunc(r.handleTOTPDisable),
			capability: "dashboard.read",
		},
	}
}

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.LoginRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}

	identity, err := r.auth.Login(w, req, strings.TrimSpace(body.Username), body.Password, body.TOTPCode)
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
		Capabilities:  identity.Capabilities,
		MFAComplete:   identity.MFAComplete,
		CSRFToken:     identity.CSRFToken,
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
		Capabilities:  identity.Capabilities,
		MFAComplete:   identity.MFAComplete,
		CSRFToken:     identity.CSRFToken,
		TTLSeconds:    int(r.auth.SessionTTL().Seconds()),
	})
}

func (r *Router) handleSessionRevoke(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.SessionRevokeRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	revoked := 0
	if body.All {
		revoked = r.auth.RevokeAllSessions()
	} else if strings.TrimSpace(body.SessionID) != "" {
		if r.auth.RevokeSessionID(strings.TrimSpace(body.SessionID)) {
			revoked = 1
		}
	} else {
		revoked = r.auth.RevokeUserSessions(strings.TrimSpace(body.Username))
	}
	targetScope := "user"
	if body.All {
		targetScope = "all"
	} else if strings.TrimSpace(body.SessionID) != "" {
		targetScope = "session"
	}
	r.logAuditRequest(req, "session.revoke", "success", strings.TrimSpace(body.Username), map[string]string{
		"revoked":      strconv.Itoa(revoked),
		"session_id":   strings.TrimSpace(body.SessionID),
		"target_scope": targetScope,
	})
	writeJSON(w, http.StatusOK, admintypes.SessionRevokeResponse{Revoked: revoked})
}

func (r *Router) handleSessions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	records := r.auth.ListSessions(strings.TrimSpace(req.URL.Query().Get("username")), req)
	out := make([]admintypes.SessionRecord, 0, len(records))
	for _, session := range records {
		out = append(out, admintypes.SessionRecord{
			ID:          session.ID,
			Username:    session.Username,
			Name:        session.Name,
			Email:       session.Email,
			Role:        session.Role,
			MFAComplete: session.MFAComplete,
			RemoteAddr:  session.RemoteAddr,
			UserAgent:   session.UserAgent,
			IssuedAt:    session.IssuedAt.UTC().Format(time.RFC3339),
			LastSeen:    session.LastSeen.UTC().Format(time.RFC3339),
			ExpiresAt:   session.ExpiresAt.UTC().Format(time.RFC3339),
			Current:     session.Current,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (r *Router) handlePasswordResetStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PasswordResetStartRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	identity, _ := adminauth.IdentityFromContext(req.Context())
	resp, err := r.auth.StartPasswordReset(identity, body.Username)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "password_reset.start", "success", body.Username, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handlePasswordResetComplete(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PasswordResetCompleteRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.auth.CompletePasswordReset(body); err != nil {
		r.logAudit(strings.TrimSpace(body.Username), "password_reset.complete", "failure", body.Username, map[string]string{"error": err.Error()})
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAudit(strings.TrimSpace(body.Username), "password_reset.complete", "success", body.Username, map[string]string{"sessions_revoked": "true"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (r *Router) handleTOTPSetup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.TOTPSetupRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	identity, _ := adminauth.IdentityFromContext(req.Context())
	resp, err := r.auth.SetupTOTP(identity, body.Username)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "totp.setup", "success", resp.Username, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleTOTPEnable(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.TOTPEnableRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	identity, _ := adminauth.IdentityFromContext(req.Context())
	if err := r.auth.EnableTOTP(identity, body.Username, body.Code); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "totp.enable", "success", body.Username, map[string]string{"sessions_revoked": "true"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (r *Router) handleTOTPDisable(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.TOTPDisableRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	identity, _ := adminauth.IdentityFromContext(req.Context())
	if err := r.auth.DisableTOTP(identity, body.Username); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "totp.disable", "success", body.Username, map[string]string{"sessions_revoked": "true"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
