package managed

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	BootstrapSignatureAlgorithm = "hmac-sha256"
	minBootstrapSecretLength    = 32
	minBootstrapNonceLength     = 16
	bootstrapStateDir           = "managed"
	bootstrapStateFile          = "bootstrap-state.json"
	bootstrapLockFile           = "bootstrap-state.lock"
)

type BootstrapPayload struct {
	WorkspaceID        string         `json:"workspace_id"`
	InstanceID         string         `json:"instance_id"`
	OwnerEmail         string         `json:"owner_email"`
	InitialSiteConfig  map[string]any `json:"initial_site_config"`
	AdminPath          string         `json:"admin_path"`
	RuntimeCallbackURL string         `json:"runtime_callback_url"`
	IssuedAt           time.Time      `json:"issued_at"`
	ExpiresAt          time.Time      `json:"expires_at"`
	Nonce              string         `json:"nonce"`
}

type SignedBootstrapPayload struct {
	Algorithm string           `json:"algorithm"`
	Payload   BootstrapPayload `json:"payload"`
	Signature string           `json:"signature"`
}

type BootstrapValidationOptions struct {
	Now time.Time
}

type BootstrapState struct {
	WorkspaceID   string    `json:"workspace_id"`
	InstanceID    string    `json:"instance_id"`
	CompletedAt   time.Time `json:"completed_at"`
	Nonce         string    `json:"nonce"`
	PayloadSHA256 string    `json:"payload_sha256"`
}

type BootstrapApplyOptions struct {
	DataDir string
	Payload BootstrapPayload
	Now     time.Time
	Apply   func(BootstrapPayload) error
}

func SignBootstrapPayload(payload BootstrapPayload, secret []byte) (SignedBootstrapPayload, error) {
	if err := validateBootstrapSecret(secret); err != nil {
		return SignedBootstrapPayload{}, err
	}
	if err := validateBootstrapPayloadFields(payload, time.Time{}); err != nil {
		return SignedBootstrapPayload{}, err
	}
	signature, err := bootstrapSignature(payload, secret)
	if err != nil {
		return SignedBootstrapPayload{}, err
	}
	return SignedBootstrapPayload{
		Algorithm: BootstrapSignatureAlgorithm,
		Payload:   payload,
		Signature: signature,
	}, nil
}

func ValidateBootstrapPayload(signed SignedBootstrapPayload, secret []byte, opts BootstrapValidationOptions) (BootstrapPayload, error) {
	if err := validateBootstrapSecret(secret); err != nil {
		return BootstrapPayload{}, err
	}
	if strings.TrimSpace(signed.Algorithm) != BootstrapSignatureAlgorithm {
		return BootstrapPayload{}, fmt.Errorf("unsupported bootstrap signature algorithm")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := validateBootstrapPayloadFields(signed.Payload, now); err != nil {
		return BootstrapPayload{}, err
	}
	expected, err := bootstrapSignature(signed.Payload, secret)
	if err != nil {
		return BootstrapPayload{}, err
	}
	actual, err := hex.DecodeString(strings.TrimSpace(signed.Signature))
	if err != nil {
		return BootstrapPayload{}, fmt.Errorf("invalid bootstrap signature")
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return BootstrapPayload{}, fmt.Errorf("invalid bootstrap signature")
	}
	if !hmac.Equal(actual, expectedBytes) {
		return BootstrapPayload{}, fmt.Errorf("invalid bootstrap signature")
	}
	return signed.Payload, nil
}

func ApplyBootstrapOnce(opts BootstrapApplyOptions) (BootstrapState, error) {
	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		return BootstrapState{}, fmt.Errorf("bootstrap data directory is required")
	}
	if opts.Apply == nil {
		return BootstrapState{}, fmt.Errorf("bootstrap apply function is required")
	}
	if err := validateBootstrapPayloadFields(opts.Payload, time.Time{}); err != nil {
		return BootstrapState{}, err
	}

	statePath, lockPath, err := bootstrapStatePaths(dataDir)
	if err != nil {
		return BootstrapState{}, err
	}
	if existing, err := ReadBootstrapState(dataDir); err == nil {
		return existing, fmt.Errorf("bootstrap has already completed")
	} else if !os.IsNotExist(err) {
		return BootstrapState{}, err
	}

	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return BootstrapState{}, fmt.Errorf("create bootstrap state directory: %w", err)
	}
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return BootstrapState{}, fmt.Errorf("bootstrap is already in progress")
		}
		return BootstrapState{}, fmt.Errorf("create bootstrap lock: %w", err)
	}
	_ = lock.Close()
	defer func() {
		_ = os.Remove(lockPath)
	}()

	if _, err := os.Stat(statePath); err == nil {
		state, readErr := ReadBootstrapState(dataDir)
		if readErr != nil {
			return BootstrapState{}, readErr
		}
		return state, fmt.Errorf("bootstrap has already completed")
	} else if !os.IsNotExist(err) {
		return BootstrapState{}, fmt.Errorf("inspect bootstrap state: %w", err)
	}

	if err := opts.Apply(opts.Payload); err != nil {
		return BootstrapState{}, fmt.Errorf("apply bootstrap payload: %w", err)
	}

	completedAt := opts.Now
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	hash, err := bootstrapPayloadHash(opts.Payload)
	if err != nil {
		return BootstrapState{}, err
	}
	state := BootstrapState{
		WorkspaceID:   strings.TrimSpace(opts.Payload.WorkspaceID),
		InstanceID:    strings.TrimSpace(opts.Payload.InstanceID),
		CompletedAt:   completedAt.UTC(),
		Nonce:         strings.TrimSpace(opts.Payload.Nonce),
		PayloadSHA256: hash,
	}
	if err := writeBootstrapState(statePath, state); err != nil {
		return BootstrapState{}, err
	}
	return state, nil
}

func ReadBootstrapState(dataDir string) (BootstrapState, error) {
	statePath, _, err := bootstrapStatePaths(dataDir)
	if err != nil {
		return BootstrapState{}, err
	}
	b, err := os.ReadFile(statePath)
	if err != nil {
		return BootstrapState{}, err
	}
	var state BootstrapState
	if err := json.Unmarshal(b, &state); err != nil {
		return BootstrapState{}, fmt.Errorf("decode bootstrap state: %w", err)
	}
	if strings.TrimSpace(state.WorkspaceID) == "" || strings.TrimSpace(state.InstanceID) == "" || state.CompletedAt.IsZero() {
		return BootstrapState{}, fmt.Errorf("bootstrap state is incomplete")
	}
	return state, nil
}

func validateBootstrapPayloadFields(payload BootstrapPayload, now time.Time) error {
	if strings.TrimSpace(payload.WorkspaceID) == "" {
		return fmt.Errorf("bootstrap payload missing workspace_id")
	}
	if strings.TrimSpace(payload.InstanceID) == "" {
		return fmt.Errorf("bootstrap payload missing instance_id")
	}
	if strings.TrimSpace(payload.OwnerEmail) == "" {
		return fmt.Errorf("bootstrap payload missing owner_email")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(payload.OwnerEmail)); err != nil {
		return fmt.Errorf("bootstrap payload has invalid owner_email")
	}
	if len(payload.InitialSiteConfig) == 0 {
		return fmt.Errorf("bootstrap payload missing initial_site_config")
	}
	if err := validateBootstrapAdminPath(payload.AdminPath); err != nil {
		return err
	}
	if err := validateBootstrapCallbackURL(payload.RuntimeCallbackURL); err != nil {
		return err
	}
	if payload.IssuedAt.IsZero() {
		return fmt.Errorf("bootstrap payload missing issued_at")
	}
	if payload.ExpiresAt.IsZero() {
		return fmt.Errorf("bootstrap payload missing expires_at")
	}
	if !payload.ExpiresAt.After(payload.IssuedAt) {
		return fmt.Errorf("bootstrap payload expires_at must be after issued_at")
	}
	if strings.TrimSpace(payload.Nonce) == "" {
		return fmt.Errorf("bootstrap payload missing nonce")
	}
	if len(strings.TrimSpace(payload.Nonce)) < minBootstrapNonceLength {
		return fmt.Errorf("bootstrap payload nonce is too short")
	}
	if !now.IsZero() {
		if payload.IssuedAt.After(now) {
			return fmt.Errorf("bootstrap payload issued_at is in the future")
		}
		if !now.Before(payload.ExpiresAt) {
			return fmt.Errorf("bootstrap payload has expired")
		}
	}
	return nil
}

func validateBootstrapSecret(secret []byte) error {
	if len(secret) < minBootstrapSecretLength {
		return fmt.Errorf("bootstrap signing secret is too short")
	}
	return nil
}

func validateBootstrapAdminPath(value string) error {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if value == "" {
		return fmt.Errorf("bootstrap payload missing admin_path")
	}
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("bootstrap payload admin_path must start with '/'")
	}
	clean := path.Clean(value)
	if clean == "/" || clean != value {
		return fmt.Errorf("bootstrap payload admin_path must be normalized and not root")
	}
	return nil
}

func validateBootstrapCallbackURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("bootstrap payload missing runtime_callback_url")
	}
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Host == "" {
		return fmt.Errorf("bootstrap payload has invalid runtime_callback_url")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("bootstrap payload runtime_callback_url must use http or https")
	}
	return nil
}

func bootstrapSignature(payload BootstrapPayload, secret []byte) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal bootstrap payload: %w", err)
	}
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write(body); err != nil {
		return "", fmt.Errorf("sign bootstrap payload: %w", err)
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func bootstrapPayloadHash(payload BootstrapPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal bootstrap payload: %w", err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func bootstrapStatePaths(dataDir string) (string, string, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return "", "", fmt.Errorf("bootstrap data directory is required")
	}
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve bootstrap data directory: %w", err)
	}
	stateDir := filepath.Join(root, bootstrapStateDir)
	return filepath.Join(stateDir, bootstrapStateFile), filepath.Join(stateDir, bootstrapLockFile), nil
}

func writeBootstrapState(path string, state BootstrapState) error {
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bootstrap state: %w", err)
	}
	body = append(body, '\n')
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0o600); err != nil {
		return fmt.Errorf("write bootstrap state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit bootstrap state: %w", err)
	}
	return nil
}
