package managed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegisterRuntimeSkipsOutsideManagedMode(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result, err := RegisterRuntime(context.Background(), RuntimeRegistrationOptions{
		Managed:     false,
		CallbackURL: server.URL,
		Secret:      []byte(strings.Repeat("s", 32)),
		Payload:     validRuntimeRegistrationPayload(time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("register runtime: %v", err)
	}
	if !result.Skipped {
		t.Fatal("expected registration to be skipped")
	}
	if called {
		t.Fatal("expected skipped registration not to call callback URL")
	}
}

func TestRegisterRuntimePostsSignedRegistration(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	secret := []byte(strings.Repeat("s", 32))
	var got RuntimeRegistrationPayload
	var gotSignature string
	var gotTimestamp string
	var gotEvent string
	var rawBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json content type, got %q", ct)
		}
		gotSignature = r.Header.Get(RuntimeSignatureHeader)
		gotTimestamp = r.Header.Get(RuntimeTimestampHeader)
		gotEvent = r.Header.Get(RuntimeEventHeader)
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		var err error
		rawBody, err = json.Marshal(got)
		if err != nil {
			t.Fatalf("remarshal body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result, err := RegisterRuntime(context.Background(), RuntimeRegistrationOptions{
		Managed:     true,
		CallbackURL: server.URL,
		Secret:      secret,
		Payload:     validRuntimeRegistrationPayload(now),
		Now:         now,
	})
	if err != nil {
		t.Fatalf("register runtime: %v", err)
	}
	if result.Skipped || result.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected registration result: %#v", result)
	}
	if got.WorkspaceID != "workspace-123" || got.InstanceID != "instance-456" {
		t.Fatalf("unexpected registration body: %#v", got)
	}
	if got.Version != "v1.3.9" || got.Commit != "abc1234" {
		t.Fatalf("expected version and commit in body, got %#v", got)
	}
	if got.SiteURL != "https://site.example.com" || got.AdminURL != "https://site.example.com/__admin" {
		t.Fatalf("expected site and admin URLs in body, got %#v", got)
	}
	if got.HealthStatus != "healthy" {
		t.Fatalf("expected health status in body, got %#v", got)
	}
	if !got.BootstrapCompletedAt.Equal(now.Add(-time.Minute)) || !got.RegisteredAt.Equal(now) {
		t.Fatalf("unexpected timestamps in body: %#v", got)
	}
	if gotEvent != RuntimeRegistrationEvent {
		t.Fatalf("expected event header %q, got %q", RuntimeRegistrationEvent, gotEvent)
	}
	if gotTimestamp != now.Format(time.RFC3339) {
		t.Fatalf("expected timestamp header %q, got %q", now.Format(time.RFC3339), gotTimestamp)
	}
	expectedSignature := runtimeSignaturePrefix + signRuntimeCallback(RuntimeRegistrationEvent, gotTimestamp, rawBody, secret)
	if gotSignature != expectedSignature {
		t.Fatalf("expected signature %q, got %q", expectedSignature, gotSignature)
	}
}

func TestRegisterRuntimeRejectsNon2xx(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "try later", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result, err := RegisterRuntime(context.Background(), RuntimeRegistrationOptions{
		Managed:     true,
		CallbackURL: server.URL,
		Secret:      []byte(strings.Repeat("s", 32)),
		Payload:     validRuntimeRegistrationPayload(now),
		Now:         now,
	})
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("expected non-2xx error, got %v", err)
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 result, got %#v", result)
	}
}

func TestRegisterRuntimeValidatesManagedInputs(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	for name, opts := range map[string]RuntimeRegistrationOptions{
		"missing callback URL": {
			Managed: true,
			Secret:  []byte(strings.Repeat("s", 32)),
			Payload: validRuntimeRegistrationPayload(now),
			Now:     now,
		},
		"short secret": {
			Managed:     true,
			CallbackURL: "https://control.example.com/runtime/register",
			Secret:      []byte("short"),
			Payload:     validRuntimeRegistrationPayload(now),
			Now:         now,
		},
		"missing instance": {
			Managed:     true,
			CallbackURL: "https://control.example.com/runtime/register",
			Secret:      []byte(strings.Repeat("s", 32)),
			Payload: func() RuntimeRegistrationPayload {
				payload := validRuntimeRegistrationPayload(now)
				payload.InstanceID = ""
				return payload
			}(),
			Now: now,
		},
		"invalid admin URL": {
			Managed:     true,
			CallbackURL: "https://control.example.com/runtime/register",
			Secret:      []byte(strings.Repeat("s", 32)),
			Payload: func() RuntimeRegistrationPayload {
				payload := validRuntimeRegistrationPayload(now)
				payload.AdminURL = "file:///tmp/admin"
				return payload
			}(),
			Now: now,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RegisterRuntime(context.Background(), opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func validRuntimeRegistrationPayload(now time.Time) RuntimeRegistrationPayload {
	return RuntimeRegistrationPayload{
		WorkspaceID:          "workspace-123",
		InstanceID:           "instance-456",
		Version:              "v1.3.9",
		Commit:               "abc1234",
		SiteURL:              "https://site.example.com",
		AdminURL:             "https://site.example.com/__admin",
		HealthStatus:         "healthy",
		BootstrapCompletedAt: now.Add(-time.Minute),
	}
}
