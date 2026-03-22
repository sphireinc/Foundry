package auth

import (
	"net/http"
	"strings"
)

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return m.WrapCapability(next, "")
}

func (m *Middleware) WrapRole(next http.Handler, requiredRole string) http.Handler {
	return m.WrapCapability(next, roleCapabilityRequirement(requiredRole))
}

func (m *Middleware) WrapCapability(next http.Handler, requiredCapability string) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := m.Authenticate(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err := m.enforceCSRF(identity, r); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if !capabilityAllowed(identity.Capabilities, requiredCapability) {
			http.Error(w, "insufficient admin capabilities", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identity)))
	})
}

func (m *Middleware) enforceCSRF(identity *Identity, r *http.Request) error {
	if r == nil || identity == nil {
		return nil
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return nil
	}
	if extractSessionToken(r) == "" || extractAccessToken(r) != "" {
		return nil
	}
	if tokensEqual(strings.TrimSpace(r.Header.Get("X-Foundry-CSRF-Token")), identity.CSRFToken) {
		return nil
	}
	return errCSRFRequired
}

func roleCapabilityRequirement(role string) string {
	switch normalizeRole(role) {
	case "admin":
		return "users.manage"
	case "editor":
		return "documents.read"
	default:
		return ""
	}
}

var errCSRFRequired = &csrfError{"csrf token is required"}

type csrfError struct{ message string }

func (e *csrfError) Error() string { return e.message }
