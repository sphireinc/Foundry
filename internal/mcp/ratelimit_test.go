package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	start := time.Unix(0, 0)
	limiter := newRateLimiter(60, 2, start)

	if !limiter.allow(start) {
		t.Fatal("expected first request to be allowed")
	}
	if !limiter.allow(start) {
		t.Fatal("expected second request to be allowed")
	}
	if limiter.allow(start) {
		t.Fatal("expected third immediate request to be blocked")
	}
	if !limiter.allow(start.Add(time.Second)) {
		t.Fatal("expected a refilled token after one second")
	}
}

func TestRateLimiterDisabledWhenZero(t *testing.T) {
	limiter := newRateLimiter(0, 0, time.Now())
	for i := 0; i < 100; i++ {
		if !limiter.allow(time.Now()) {
			t.Fatal("expected disabled limiter to allow every request")
		}
	}
}

func TestServerRateLimitRejectsExcess(t *testing.T) {
	fixed := time.Unix(0, 0)
	policy, err := NewPolicy(PolicyOptions{
		EnabledScopes: readScopes(),
		RateLimit:     RateLimit{RequestsPerMinute: 60, Burst: 2},
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	server := newTestServerWithOptions(t, ServerOptions{
		Authorizer: NewAuthorizer(policy),
		Actor:      LocalActor(),
		Now:        func() time.Time { return fixed },
	})

	responses := serve(t, server, requestBatch(
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	))

	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	if responses[0].Error != nil || responses[1].Error != nil {
		t.Fatalf("expected first two requests to succeed: %+v", responses)
	}
	if responses[2].Error == nil || responses[2].Error.Code != codeRateLimited {
		t.Fatalf("expected third request to be rate limited, got %+v", responses[2])
	}
}

func TestServerConcurrencyLimitRejectsWhenSaturated(t *testing.T) {
	policy, err := NewPolicy(PolicyOptions{
		EnabledScopes:  readScopes(),
		MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	server := newTestServerWithOptions(t, ServerOptions{
		Authorizer: NewAuthorizer(policy),
		Actor:      LocalActor(),
	})

	server.slots <- struct{}{}
	defer func() { <-server.slots }()

	_, rpcErr, _ := server.handle(context.Background(), rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      json.RawMessage("1"),
		Method:  "ping",
	})
	if rpcErr == nil || rpcErr.Code != codeServerBusy {
		t.Fatalf("expected concurrency rejection, got %+v", rpcErr)
	}
}
