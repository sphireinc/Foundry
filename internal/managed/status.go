package managed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	RuntimeDeploymentStatusEvent = "runtime.deployment.status"
	RuntimeBackupStatusEvent     = "runtime.backup.status"
	RuntimeRestoreStatusEvent    = "runtime.restore.status"
	RuntimeDomainStatusEvent     = "runtime.domain.status"
	RuntimeVersionStatusEvent    = "runtime.version.status"
)

type RuntimeStatusOptions struct {
	Managed     bool
	CallbackURL string
	Secret      []byte
	WorkspaceID string
	InstanceID  string
	Now         time.Time
	Client      *http.Client
	MaxAttempts int
	RetryDelay  time.Duration
}

type RuntimeStatusCallbackPayload struct {
	EventID     string         `json:"event_id"`
	WorkspaceID string         `json:"workspace_id"`
	InstanceID  string         `json:"instance_id"`
	EventType   string         `json:"event_type"`
	Status      string         `json:"status"`
	Message     string         `json:"message,omitempty"`
	ObservedAt  time.Time      `json:"observed_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RuntimeStatusResult struct {
	Skipped    bool
	StatusCode int
}

type RuntimeDeploymentStatus struct {
	Status       string
	Message      string
	DeploymentID string
	Version      string
	Commit       string
	Metadata     map[string]any
}

type RuntimeBackupStatus struct {
	Status       string
	Message      string
	BackupID     string
	SnapshotName string
	Metadata     map[string]any
}

type RuntimeRestoreStatus struct {
	Status    string
	Message   string
	RestoreID string
	BackupID  string
	Metadata  map[string]any
}

type RuntimeDomainStatus struct {
	Status    string
	Message   string
	Hostname  string
	DNSStatus string
	TLSStatus string
	Metadata  map[string]any
}

type RuntimeVersionStatus struct {
	Status         string
	Message        string
	CurrentVersion string
	TargetVersion  string
	UpdateID       string
	Metadata       map[string]any
}

func SendDeploymentStatus(ctx context.Context, opts RuntimeStatusOptions, status RuntimeDeploymentStatus) (RuntimeStatusResult, error) {
	metadata := copyRuntimeMetadata(status.Metadata)
	setRuntimeMetadata(metadata, "deployment_id", status.DeploymentID)
	setRuntimeMetadata(metadata, "version", status.Version)
	setRuntimeMetadata(metadata, "commit", status.Commit)
	return sendRuntimeStatus(ctx, opts, RuntimeDeploymentStatusEvent, status.Status, status.Message, metadata)
}

func SendBackupStatus(ctx context.Context, opts RuntimeStatusOptions, status RuntimeBackupStatus) (RuntimeStatusResult, error) {
	metadata := copyRuntimeMetadata(status.Metadata)
	setRuntimeMetadata(metadata, "backup_id", status.BackupID)
	setRuntimeMetadata(metadata, "snapshot_name", status.SnapshotName)
	return sendRuntimeStatus(ctx, opts, RuntimeBackupStatusEvent, status.Status, status.Message, metadata)
}

func SendRestoreStatus(ctx context.Context, opts RuntimeStatusOptions, status RuntimeRestoreStatus) (RuntimeStatusResult, error) {
	metadata := copyRuntimeMetadata(status.Metadata)
	setRuntimeMetadata(metadata, "restore_id", status.RestoreID)
	setRuntimeMetadata(metadata, "backup_id", status.BackupID)
	return sendRuntimeStatus(ctx, opts, RuntimeRestoreStatusEvent, status.Status, status.Message, metadata)
}

func SendDomainStatus(ctx context.Context, opts RuntimeStatusOptions, status RuntimeDomainStatus) (RuntimeStatusResult, error) {
	metadata := copyRuntimeMetadata(status.Metadata)
	setRuntimeMetadata(metadata, "hostname", status.Hostname)
	setRuntimeMetadata(metadata, "dns_status", status.DNSStatus)
	setRuntimeMetadata(metadata, "tls_status", status.TLSStatus)
	return sendRuntimeStatus(ctx, opts, RuntimeDomainStatusEvent, status.Status, status.Message, metadata)
}

func SendVersionStatus(ctx context.Context, opts RuntimeStatusOptions, status RuntimeVersionStatus) (RuntimeStatusResult, error) {
	metadata := copyRuntimeMetadata(status.Metadata)
	setRuntimeMetadata(metadata, "current_version", status.CurrentVersion)
	setRuntimeMetadata(metadata, "target_version", status.TargetVersion)
	setRuntimeMetadata(metadata, "update_id", status.UpdateID)
	return sendRuntimeStatus(ctx, opts, RuntimeVersionStatusEvent, status.Status, status.Message, metadata)
}

func sendRuntimeStatus(ctx context.Context, opts RuntimeStatusOptions, eventType, status, message string, metadata map[string]any) (RuntimeStatusResult, error) {
	if !opts.Managed {
		return RuntimeStatusResult{Skipped: true}, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload := RuntimeStatusCallbackPayload{
		WorkspaceID: strings.TrimSpace(opts.WorkspaceID),
		InstanceID:  strings.TrimSpace(opts.InstanceID),
		EventType:   strings.TrimSpace(eventType),
		Status:      strings.TrimSpace(status),
		Message:     strings.TrimSpace(message),
		ObservedAt:  now.UTC(),
		Metadata:    metadata,
	}
	if err := validateRuntimeStatusPayload(payload); err != nil {
		return RuntimeStatusResult{}, err
	}
	eventID, err := runtimeStatusEventID(payload)
	if err != nil {
		return RuntimeStatusResult{}, err
	}
	payload.EventID = eventID
	result, err := postSignedRuntimeCallback(ctx, runtimeSignedCallbackOptions{
		CallbackURL: opts.CallbackURL,
		Secret:      opts.Secret,
		Event:       payload.EventType,
		Now:         now,
		Client:      opts.Client,
		MaxAttempts: opts.MaxAttempts,
		RetryDelay:  opts.RetryDelay,
		Payload:     payload,
	})
	if err != nil {
		return RuntimeStatusResult{StatusCode: result.StatusCode}, err
	}
	return RuntimeStatusResult{StatusCode: result.StatusCode}, nil
}

func validateRuntimeStatusPayload(payload RuntimeStatusCallbackPayload) error {
	if strings.TrimSpace(payload.WorkspaceID) == "" {
		return fmt.Errorf("runtime status missing workspace_id")
	}
	if strings.TrimSpace(payload.InstanceID) == "" {
		return fmt.Errorf("runtime status missing instance_id")
	}
	if strings.TrimSpace(payload.EventType) == "" {
		return fmt.Errorf("runtime status missing event_type")
	}
	if strings.TrimSpace(payload.Status) == "" {
		return fmt.Errorf("runtime status missing status")
	}
	if payload.ObservedAt.IsZero() {
		return fmt.Errorf("runtime status missing observed_at")
	}
	return nil
}

func runtimeStatusEventID(payload RuntimeStatusCallbackPayload) (string, error) {
	payload.EventID = ""
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal runtime status event id: %w", err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func copyRuntimeMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+4)
	for k, v := range in {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func setRuntimeMetadata(metadata map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	metadata[key] = strings.TrimSpace(value)
}
