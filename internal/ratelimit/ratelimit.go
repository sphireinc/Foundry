// Package ratelimit provides bounded, per-client token-bucket limiting for
// HTTP handlers.
package ratelimit

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Policy configures a per-client token bucket. A non-positive value disables
// the limiter; configuration validation should reject partially configured
// policies before they reach this package.
type Policy struct {
	RequestsPerMinute int
	Burst             int
	MaxClients        int
}

// Limiter tracks bounded token buckets for a single traffic class.
type Limiter struct {
	policy  Policy
	now     func() time.Time
	mu      sync.Mutex
	clients map[string]clientBucket
}

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// New returns a limiter for policy or nil when policy is disabled.
func New(policy Policy) *Limiter {
	if policy.RequestsPerMinute <= 0 || policy.Burst <= 0 || policy.MaxClients <= 0 {
		return nil
	}
	return newLimiter(policy, time.Now)
}

func newLimiter(policy Policy, now func() time.Time) *Limiter {
	return &Limiter{
		policy:  policy,
		now:     now,
		clients: make(map[string]clientBucket),
	}
}

// Allow reports whether key may make a request. When denied, retryAfter is
// rounded up to whole seconds for use in a Retry-After response header.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	if l == nil {
		return true, 0
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.clients[key]
	if !ok {
		if len(l.clients) >= l.policy.MaxClients {
			l.evictIdle(now)
		}
		if len(l.clients) >= l.policy.MaxClients {
			return false, l.nextTokenDelay()
		}
		bucket = clientBucket{
			tokens:     float64(l.policy.Burst),
			lastRefill: now,
			lastSeen:   now,
		}
	}

	if elapsed := now.Sub(bucket.lastRefill); elapsed > 0 {
		tokens := bucket.tokens + elapsed.Minutes()*float64(l.policy.RequestsPerMinute)
		bucket.tokens = math.Min(float64(l.policy.Burst), tokens)
		bucket.lastRefill = now
	}
	bucket.lastSeen = now

	if bucket.tokens < 1 {
		l.clients[key] = bucket
		return false, l.nextTokenDelayFor(bucket.tokens)
	}
	bucket.tokens--
	l.clients[key] = bucket
	return true, 0
}

func (l *Limiter) evictIdle(now time.Time) {
	deadline := now.Add(-l.idleTTL())
	for key, bucket := range l.clients {
		if bucket.lastSeen.Before(deadline) {
			delete(l.clients, key)
		}
	}
}

func (l *Limiter) idleTTL() time.Duration {
	refillDuration := time.Duration(float64(time.Minute) * float64(l.policy.Burst) / float64(l.policy.RequestsPerMinute))
	if refillDuration < time.Minute {
		refillDuration = time.Minute
	}
	if refillDuration > 12*time.Hour {
		refillDuration = 12 * time.Hour
	}
	return 2 * refillDuration
}

func (l *Limiter) nextTokenDelay() time.Duration {
	return l.nextTokenDelayFor(0)
}

func (l *Limiter) nextTokenDelayFor(tokens float64) time.Duration {
	seconds := (1 - tokens) * 60 / float64(l.policy.RequestsPerMinute)
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(math.Ceil(seconds)) * time.Second
}

// ParseTrustedProxies parses configured CIDR prefixes. Configuration validation
// reports malformed values; this helper ignores them defensively for callers
// that construct Config values directly.
func ParseTrustedProxies(values []string) []netip.Prefix {
	trusted := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err == nil {
			trusted = append(trusted, prefix.Masked())
		}
	}
	return trusted
}

// ClientKey returns the source IP for request rate limiting. Forwarded client
// headers are considered only when the direct peer is a configured trusted
// proxy, preventing clients from choosing their own rate-limit key.
func ClientKey(r *http.Request, trusted []netip.Prefix) string {
	if r == nil {
		return "unknown"
	}
	direct, ok := parseRemoteAddr(r.RemoteAddr)
	if !ok {
		return "unknown"
	}
	if !isTrusted(direct, trusted) {
		return direct.String()
	}

	chain := forwardedFor(r.Header.Values("X-Forwarded-For"))
	chain = append(chain, direct)
	for i := len(chain) - 1; i >= 0; i-- {
		if !isTrusted(chain[i], trusted) {
			return chain[i].String()
		}
	}
	return direct.String()
}

func parseRemoteAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func forwardedFor(headers []string) []netip.Addr {
	var addresses []netip.Addr
	for _, header := range headers {
		for _, value := range strings.Split(header, ",") {
			addr, err := netip.ParseAddr(strings.TrimSpace(value))
			if err == nil {
				addresses = append(addresses, addr.Unmap())
			}
		}
	}
	return addresses
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
