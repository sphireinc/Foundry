package auth

import (
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
	return nil
}

func isLocalRequest(r *http.Request) bool {
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
