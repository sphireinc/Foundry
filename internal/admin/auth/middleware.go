package auth

import "net/http"

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := m.Authenticate(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identity)))
	})
}

func (m *Middleware) WrapRole(next http.Handler, requiredRole string) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := m.Authenticate(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if !roleAllowed(identity.Role, requiredRole) {
			http.Error(w, "admin role is required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identity)))
	})
}
