package mcp

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newRateLimiter(perMinute, burst int, now time.Time) *rateLimiter {
	if perMinute <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = perMinute
	}
	return &rateLimiter{
		rate:   float64(perMinute) / 60,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now,
	}
}

func (l *rateLimiter) allow(now time.Time) bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.last) {
		l.tokens += now.Sub(l.last).Seconds() * l.rate
	}
	l.last = now
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
