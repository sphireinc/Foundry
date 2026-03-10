package auth

import "net/http"

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.Authorize(r); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
