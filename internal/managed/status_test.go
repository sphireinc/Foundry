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

func TestRuntimeStatusCallbacksPostSignedPayloads(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	secret := []byte(strings.Repeat("s", 32))

	tests := []struct {
		name     string
		event    string
		send     func(RuntimeStatusOptions) (RuntimeStatusResult, error)
		metadata map[string]string
	}{
		{
			name:  "deployment",
			event: RuntimeDeploymentStatusEvent,
			send: func(opts RuntimeStatusOptions) (RuntimeStatusResult, error) {
				return SendDeploymentStatus(context.Background(), opts, RuntimeDeploymentStatus{
					Status:       "succeeded",
					Message:      "deployment completed",
					DeploymentID: "deployment-123",
					Version:      "v1.3.9",
					Commit:       "abc1234",
					Metadata:     map[string]any{"duration_ms": 1200},
				})
			},
			metadata: map[string]string{
				"deployment_id": "deployment-123",
				"version":       "v1.3.9",
				"commit":        "abc1234",
			},
		},
		{
			name:  "backup",
			event: RuntimeBackupStatusEvent,
			send: func(opts RuntimeStatusOptions) (RuntimeStatusResult, error) {
				return SendBackupStatus(context.Background(), opts, RuntimeBackupStatus{
					Status:       "succeeded",
					Message:      "backup completed",
					BackupID:     "backup-123",
					SnapshotName: "snapshot.zip",
				})
			},
			metadata: map[string]string{
				"backup_id":     "backup-123",
				"snapshot_name": "snapshot.zip",
			},
		},
		{
			name:  "restore",
			event: RuntimeRestoreStatusEvent,
			send: func(opts RuntimeStatusOptions) (RuntimeStatusResult, error) {
				return SendRestoreStatus(context.Background(), opts, RuntimeRestoreStatus{
					Status:    "running",
					Message:   "restore started",
					RestoreID: "restore-123",
					BackupID:  "backup-123",
				})
			},
			metadata: map[string]string{
				"restore_id": "restore-123",
				"backup_id":  "backup-123",
			},
		},
		{
			name:  "domain",
			event: RuntimeDomainStatusEvent,
			send: func(opts RuntimeStatusOptions) (RuntimeStatusResult, error) {
				return SendDomainStatus(context.Background(), opts, RuntimeDomainStatus{
					Status:    "ready",
					Message:   "domain verified",
					Hostname:  "site.example.com",
					DNSStatus: "verified",
					TLSStatus: "active",
				})
			},
			metadata: map[string]string{
				"hostname":   "site.example.com",
				"dns_status": "verified",
				"tls_status": "active",
			},
		},
		{
			name:  "version",
			event: RuntimeVersionStatusEvent,
			send: func(opts RuntimeStatusOptions) (RuntimeStatusResult, error) {
				return SendVersionStatus(context.Background(), opts, RuntimeVersionStatus{
					Status:         "available",
					Message:        "update available",
					CurrentVersion: "v1.3.9",
					TargetVersion:  "v1.4.0",
					UpdateID:       "update-123",
				})
			},
			metadata: map[string]string{
				"current_version": "v1.3.9",
				"target_version":  "v1.4.0",
				"update_id":       "update-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload RuntimeStatusCallbackPayload
			var gotSignature string
			var gotTimestamp string
			var gotEvent string
			var rawBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotSignature = r.Header.Get(RuntimeSignatureHeader)
				gotTimestamp = r.Header.Get(RuntimeTimestampHeader)
				gotEvent = r.Header.Get(RuntimeEventHeader)
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				var err error
				rawBody, err = json.Marshal(payload)
				if err != nil {
					t.Fatalf("remarshal payload: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			result, err := tt.send(RuntimeStatusOptions{
				Managed:     true,
				CallbackURL: server.URL,
				Secret:      secret,
				WorkspaceID: "workspace-123",
				InstanceID:  "instance-456",
				Now:         now,
				MaxAttempts: 1,
			})
			if err != nil {
				t.Fatalf("send status: %v", err)
			}
			if result.Skipped || result.StatusCode != http.StatusNoContent {
				t.Fatalf("unexpected result: %#v", result)
			}
			if payload.EventID == "" {
				t.Fatal("expected event_id")
			}
			if payload.WorkspaceID != "workspace-123" || payload.InstanceID != "instance-456" {
				t.Fatalf("unexpected runtime IDs: %#v", payload)
			}
			if payload.EventType != tt.event || gotEvent != tt.event {
				t.Fatalf("expected event %q, got body=%q header=%q", tt.event, payload.EventType, gotEvent)
			}
			if payload.Status == "" || payload.Message == "" {
				t.Fatalf("expected status and message, got %#v", payload)
			}
			if !payload.ObservedAt.Equal(now) {
				t.Fatalf("expected observed_at %s, got %s", now, payload.ObservedAt)
			}
			for key, want := range tt.metadata {
				if got, _ := payload.Metadata[key].(string); got != want {
					t.Fatalf("expected metadata %s=%q, got %#v", key, want, payload.Metadata[key])
				}
			}
			if gotTimestamp != now.Format(time.RFC3339) {
				t.Fatalf("expected timestamp %q, got %q", now.Format(time.RFC3339), gotTimestamp)
			}
			expectedSignature := runtimeSignaturePrefix + signRuntimeCallback(tt.event, gotTimestamp, rawBody, secret)
			if gotSignature != expectedSignature {
				t.Fatalf("expected signature %q, got %q", expectedSignature, gotSignature)
			}
		})
	}
}

func TestRuntimeStatusSkipsOutsideManagedMode(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result, err := SendBackupStatus(context.Background(), RuntimeStatusOptions{
		Managed:     false,
		CallbackURL: server.URL,
		Secret:      []byte(strings.Repeat("s", 32)),
		WorkspaceID: "workspace-123",
		InstanceID:  "instance-456",
	}, RuntimeBackupStatus{Status: "succeeded", BackupID: "backup-123"})
	if err != nil {
		t.Fatalf("send skipped status: %v", err)
	}
	if !result.Skipped {
		t.Fatal("expected skipped result")
	}
	if called {
		t.Fatal("expected skipped status not to call callback URL")
	}
}

func TestRuntimeStatusRetriesRetryableFailures(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	secret := []byte(strings.Repeat("s", 32))
	attempts := 0
	var firstEventID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var payload RuntimeStatusCallbackPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if firstEventID == "" {
			firstEventID = payload.EventID
		} else if payload.EventID != firstEventID {
			t.Fatalf("expected stable event ID across retries, got %q then %q", firstEventID, payload.EventID)
		}
		if attempts == 1 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result, err := SendDeploymentStatus(context.Background(), RuntimeStatusOptions{
		Managed:     true,
		CallbackURL: server.URL,
		Secret:      secret,
		WorkspaceID: "workspace-123",
		InstanceID:  "instance-456",
		Now:         now,
		MaxAttempts: 2,
		RetryDelay:  time.Nanosecond,
	}, RuntimeDeploymentStatus{Status: "succeeded", DeploymentID: "deployment-123"})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if result.StatusCode != http.StatusNoContent || attempts != 2 {
		t.Fatalf("expected second attempt success, result=%#v attempts=%d", result, attempts)
	}
}

func TestRuntimeStatusRejectsInvalidPayload(t *testing.T) {
	_, err := SendDomainStatus(context.Background(), RuntimeStatusOptions{
		Managed:     true,
		CallbackURL: "https://control.example.com/runtime/status",
		Secret:      []byte(strings.Repeat("s", 32)),
		WorkspaceID: "workspace-123",
		InstanceID:  "instance-456",
	}, RuntimeDomainStatus{Hostname: "site.example.com"})
	if err == nil || !strings.Contains(err.Error(), "missing status") {
		t.Fatalf("expected missing status validation error, got %v", err)
	}
}
