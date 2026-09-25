package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterAllowsBurstThenRefills(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{RequestsPerMinute: 60, Burst: 2, MaxClients: 10}, func() time.Time { return now })

	for i := 0; i < 2; i++ {
		if allowed, retryAfter := limiter.Allow("192.0.2.10"); !allowed || retryAfter != 0 {
			t.Fatalf("request %d: expected burst request to pass, allowed=%t retry_after=%s", i+1, allowed, retryAfter)
		}
	}
	if allowed, retryAfter := limiter.Allow("192.0.2.10"); allowed || retryAfter != time.Second {
		t.Fatalf("expected third request to be limited for one second, allowed=%t retry_after=%s", allowed, retryAfter)
	}

	now = now.Add(time.Second)
	if allowed, retryAfter := limiter.Allow("192.0.2.10"); !allowed || retryAfter != 0 {
		t.Fatalf("expected token to refill, allowed=%t retry_after=%s", allowed, retryAfter)
	}
}

func TestLimiterSeparatesClientsAndBoundsTracking(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{RequestsPerMinute: 60, Burst: 1, MaxClients: 2}, func() time.Time { return now })

	for _, client := range []string{"192.0.2.1", "192.0.2.2"} {
		if allowed, _ := limiter.Allow(client); !allowed {
			t.Fatalf("expected %s to receive an independent token", client)
		}
	}
	if allowed, retryAfter := limiter.Allow("192.0.2.3"); allowed || retryAfter != time.Second {
		t.Fatalf("expected client table overflow to be limited, allowed=%t retry_after=%s", allowed, retryAfter)
	}

	now = now.Add(3 * time.Minute)
	if allowed, _ := limiter.Allow("192.0.2.3"); !allowed {
		t.Fatal("expected idle client buckets to be evicted")
	}
}

func TestClientKeyUsesForwardedAddressOnlyForTrustedProxies(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.20")

	if got := ClientKey(request, nil); got != "198.51.100.10" {
		t.Fatalf("expected direct client to ignore forwarded header, got %q", got)
	}
	trusted := ParseTrustedProxies([]string{"198.51.100.0/24"})
	if got := ClientKey(request, trusted); got != "203.0.113.20" {
		t.Fatalf("expected trusted proxy to use forwarded client, got %q", got)
	}
}

func TestClientKeyUsesRightmostUntrustedForwardedAddress(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	request.RemoteAddr = "10.0.0.2:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.20, 192.0.2.7")
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8", "192.0.2.0/24"})

	if got := ClientKey(request, trusted); got != "203.0.113.20" {
		t.Fatalf("expected rightmost untrusted client, got %q", got)
	}
}
