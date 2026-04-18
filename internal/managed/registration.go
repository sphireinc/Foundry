package managed

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RuntimeRegistrationEvent  = "runtime.registration"
	RuntimeSignatureHeader    = "X-Foundry-Signature"
	RuntimeTimestampHeader    = "X-Foundry-Timestamp"
	RuntimeEventHeader        = "X-Foundry-Event"
	runtimeSignaturePrefix    = "hmac-sha256="
	defaultRuntimeHTTPTimeout = 10 * time.Second
)

type RuntimeRegistrationPayload struct {
	WorkspaceID          string    `json:"workspace_id"`
	InstanceID           string    `json:"instance_id"`
	Version              string    `json:"version"`
	Commit               string    `json:"commit"`
	SiteURL              string    `json:"site_url"`
	AdminURL             string    `json:"admin_url"`
	HealthStatus         string    `json:"health_status"`
	BootstrapCompletedAt time.Time `json:"bootstrap_completed_at"`
	RegisteredAt         time.Time `json:"registered_at"`
}

type RuntimeRegistrationOptions struct {
	Managed     bool
	CallbackURL string
	Secret      []byte
	Payload     RuntimeRegistrationPayload
	Now         time.Time
	Client      *http.Client
	MaxAttempts int
	RetryDelay  time.Duration
}

type RuntimeRegistrationResult struct {
	Skipped    bool
	StatusCode int
}

func RegisterRuntime(ctx context.Context, opts RuntimeRegistrationOptions) (RuntimeRegistrationResult, error) {
	if !opts.Managed {
		return RuntimeRegistrationResult{Skipped: true}, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload := opts.Payload
	payload.RegisteredAt = now.UTC()
	if err := validateRuntimeRegistrationPayload(payload); err != nil {
		return RuntimeRegistrationResult{}, err
	}
	result, err := postSignedRuntimeCallback(ctx, runtimeSignedCallbackOptions{
		CallbackURL: opts.CallbackURL,
		Secret:      opts.Secret,
		Event:       RuntimeRegistrationEvent,
		Now:         now,
		Client:      opts.Client,
		MaxAttempts: opts.MaxAttempts,
		RetryDelay:  opts.RetryDelay,
		Payload:     payload,
	})
	if err != nil {
		return RuntimeRegistrationResult{StatusCode: result.StatusCode}, err
	}
	return RuntimeRegistrationResult{StatusCode: result.StatusCode}, nil
}

func validateRuntimeRegistrationPayload(payload RuntimeRegistrationPayload) error {
	if strings.TrimSpace(payload.WorkspaceID) == "" {
		return fmt.Errorf("runtime registration missing workspace_id")
	}
	if strings.TrimSpace(payload.InstanceID) == "" {
		return fmt.Errorf("runtime registration missing instance_id")
	}
	if strings.TrimSpace(payload.Version) == "" {
		return fmt.Errorf("runtime registration missing version")
	}
	if strings.TrimSpace(payload.Commit) == "" {
		return fmt.Errorf("runtime registration missing commit")
	}
	if err := validateRuntimeURL("site_url", payload.SiteURL); err != nil {
		return err
	}
	if err := validateRuntimeURL("admin_url", payload.AdminURL); err != nil {
		return err
	}
	if strings.TrimSpace(payload.HealthStatus) == "" {
		return fmt.Errorf("runtime registration missing health_status")
	}
	if payload.BootstrapCompletedAt.IsZero() {
		return fmt.Errorf("runtime registration missing bootstrap_completed_at")
	}
	if payload.RegisteredAt.IsZero() {
		return fmt.Errorf("runtime registration missing registered_at")
	}
	return nil
}

func validateRuntimeCallbackURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("runtime callback URL is required")
	}
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Host == "" {
		return "", fmt.Errorf("runtime callback URL is invalid")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("runtime callback URL must use http or https")
	}
	return u.String(), nil
}

func validateRuntimeURL(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("runtime registration missing %s", name)
	}
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Host == "" {
		return fmt.Errorf("runtime registration has invalid %s", name)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("runtime registration %s must use http or https", name)
	}
	return nil
}

func signRuntimeCallback(event, timestamp string, body, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(timestamp)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(strings.TrimSpace(event)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
