package auth

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

type Middleware struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Middleware {
	return &Middleware{cfg: cfg}
}

func (m *Middleware) Authorize(r *http.Request) error {
	if m == nil || m.cfg == nil {
		return nil
	}
	if !m.cfg.Admin.Enabled {
		return fmt.Errorf("admin is disabled")
	}
	if m.cfg.Admin.LocalOnly && !isLocalRequest(r) {
		return fmt.Errorf("admin is restricted to local requests")
	}
	token := strings.TrimSpace(m.cfg.Admin.AccessToken)
	if token == "" {
		return fmt.Errorf("admin access token is required")
	}
	if !tokensEqual(extractAccessToken(r), token) {
		return fmt.Errorf("invalid admin access token")
	}
	return nil
}

func isLocalRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if hasProxyHeaders(r) {
		return false
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}

	return ip.IsLoopback()
}

func hasProxyHeaders(r *http.Request) bool {
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
		if strings.TrimSpace(r.Header.Get(name)) != "" {
			return true
		}
	}
	return false
}

func extractAccessToken(r *http.Request) string {
	if r == nil {
		return ""
	}

	if token := strings.TrimSpace(r.Header.Get("X-Foundry-Admin-Token")); token != "" {
		return token
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(authz, bearerPrefix) {
		return strings.TrimSpace(authz[len(bearerPrefix):])
	}

	return ""
}

func tokensEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
