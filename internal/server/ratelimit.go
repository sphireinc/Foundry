package server

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/ratelimit"
)

type rateLimitScope uint8

const (
	rateLimitPublic rateLimitScope = iota
	rateLimitAdmin
	rateLimitAdminAPI
)

func (s *Server) wrapRateLimit(next http.Handler) http.Handler {
	if next == nil || s == nil || s.cfg == nil {
		return next
	}

	limiters := map[rateLimitScope]*ratelimit.Limiter{
		rateLimitPublic:   newRateLimiter(s.cfg.Server.RateLimit.Public),
		rateLimitAdmin:    newRateLimiter(s.cfg.Server.RateLimit.Admin),
		rateLimitAdminAPI: newRateLimiter(s.cfg.Server.RateLimit.AdminAPI),
	}
	if limiters[rateLimitPublic] == nil && limiters[rateLimitAdmin] == nil && limiters[rateLimitAdminAPI] == nil {
		return next
	}

	trustedProxies := ratelimit.ParseTrustedProxies(s.cfg.Server.TrustedProxies)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := s.rateLimitScope(r.URL.Path)
		limiter := limiters[scope]
		if limiter == nil {
			next.ServeHTTP(w, r)
			return
		}
		if allowed, retryAfter := limiter.Allow(ratelimit.ClientKey(r, trustedProxies)); allowed {
			next.ServeHTTP(w, r)
			return
		} else {
			writeRateLimitResponse(w, scope, retryAfter)
		}
	})
}

func newRateLimiter(policy config.RateLimitPolicy) *ratelimit.Limiter {
	return ratelimit.New(ratelimit.Policy{
		RequestsPerMinute: policy.RequestsPerMinute,
		Burst:             policy.Burst,
		MaxClients:        policy.MaxClients,
	})
}

func (s *Server) rateLimitScope(requestPath string) rateLimitScope {
	adminPath := s.cfg.AdminPath()
	for _, apiPrefix := range []string{adminPath + "/api", adminPath + "/plugin-api"} {
		if requestPath == apiPrefix || strings.HasPrefix(requestPath, apiPrefix+"/") {
			return rateLimitAdminAPI
		}
	}
	if requestPath == adminPath || strings.HasPrefix(requestPath, adminPath+"/") {
		return rateLimitAdmin
	}
	return rateLimitPublic
}

func writeRateLimitResponse(w http.ResponseWriter, scope rateLimitScope, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", strconv.Itoa(seconds))

	if scope == rateLimitAdminAPI {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "rate_limited",
			"message": "too many requests; try again later",
		})
		return
	}
	http.Error(w, "too many requests; try again later", http.StatusTooManyRequests)
}
