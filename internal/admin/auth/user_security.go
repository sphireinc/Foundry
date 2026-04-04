package auth

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
)

func (m *Middleware) StartPasswordReset(identity *Identity, username string) (*types.PasswordResetStartResponse, error) {
	if m == nil || m.cfg == nil {
		return nil, fmt.Errorf("admin auth is not configured")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if identity == nil {
		return nil, fmt.Errorf("admin identity is required")
	}
	if !strings.EqualFold(identity.Username, username) && !capabilityAllowed(identity.Capabilities, "users.manage") {
		return nil, fmt.Errorf("password reset is not allowed for this user")
	}

	all, err := users.Load(m.cfg.Admin.UsersFile)
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	tokenHash, err := users.HashPassword(token)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(time.Duration(m.cfg.Admin.PasswordResetTTL) * time.Minute)
	found := false
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
			all[i].ResetTokenHash = tokenHash
			all[i].ResetTokenExpires = expiresAt
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	if err := users.Save(m.cfg.Admin.UsersFile, all); err != nil {
		return nil, err
	}
	return &types.PasswordResetStartResponse{
		Username:   username,
		ResetToken: token,
		ExpiresIn:  int(time.Until(expiresAt).Seconds()),
	}, nil
}

func (m *Middleware) CompletePasswordReset(req types.PasswordResetCompleteRequest) error {
	if m == nil || m.cfg == nil {
		return fmt.Errorf("admin auth is not configured")
	}
	if err := ValidatePassword(m.cfg, req.NewPassword); err != nil {
		return err
	}
	all, err := users.Load(m.cfg.Admin.UsersFile)
	if err != nil {
		return err
	}
	username := strings.TrimSpace(req.Username)
	found := false
	now := time.Now().UTC()
	for i := range all {
		if !strings.EqualFold(all[i].Username, username) {
			continue
		}
		if all[i].ResetTokenHash == "" || now.After(all[i].ResetTokenExpires) || !users.VerifyPassword(all[i].ResetTokenHash, req.ResetToken) {
			return fmt.Errorf("invalid or expired reset token")
		}
		plainSecret, migratedSecret, err := m.decodeTOTPSecret(all[i].TOTPSecret)
		if err != nil {
			return fmt.Errorf("two-factor authentication is not available")
		}
		if all[i].TOTPEnabled && !VerifyTOTP(plainSecret, req.TOTPCode, now) {
			return fmt.Errorf("valid two-factor code is required")
		}
		hash, err := users.HashPassword(req.NewPassword)
		if err != nil {
			return err
		}
		all[i].PasswordHash = hash
		if migratedSecret != "" {
			all[i].TOTPSecret = migratedSecret
		}
		all[i].ResetTokenHash = ""
		all[i].ResetTokenExpires = time.Time{}
		found = true
		break
	}
	if !found {
		return fmt.Errorf("user not found: %s", username)
	}
	if err := users.Save(m.cfg.Admin.UsersFile, all); err != nil {
		return err
	}
	m.RevokeUserSessions(username)
	return nil
}

func (m *Middleware) SetupTOTP(identity *Identity, username string) (*types.TOTPSetupResponse, error) {
	if m == nil || m.cfg == nil {
		return nil, fmt.Errorf("admin auth is not configured")
	}
	username = resolveTargetUsername(identity, username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if err := allowSameUserOrAdmin(identity, username); err != nil {
		return nil, err
	}
	all, err := users.Load(m.cfg.Admin.UsersFile)
	if err != nil {
		return nil, err
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return nil, err
	}
	encryptedSecret, err := m.encryptTOTPSecret(secret)
	if err != nil {
		return nil, err
	}
	found := false
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
			all[i].TOTPSecret = encryptedSecret
			all[i].TOTPEnabled = false
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	if err := users.Save(m.cfg.Admin.UsersFile, all); err != nil {
		return nil, err
	}
	return &types.TOTPSetupResponse{
		Username:        username,
		Secret:          secret,
		ProvisioningURI: TOTPProvisioningURI(m.cfg.Admin.TOTPIssuer, username, secret),
	}, nil
}

func (m *Middleware) EnableTOTP(identity *Identity, username, code string) error {
	username = resolveTargetUsername(identity, username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if err := allowSameUserOrAdmin(identity, username); err != nil {
		return err
	}
	all, err := users.Load(m.cfg.Admin.UsersFile)
	if err != nil {
		return err
	}
	found := false
	for i := range all {
		if !strings.EqualFold(all[i].Username, username) {
			continue
		}
		if all[i].TOTPSecret == "" {
			return fmt.Errorf("two-factor authentication has not been set up")
		}
		plainSecret, migratedSecret, err := m.decodeTOTPSecret(all[i].TOTPSecret)
		if err != nil {
			return fmt.Errorf("two-factor authentication is not available")
		}
		if !VerifyTOTP(plainSecret, code, time.Now()) {
			return fmt.Errorf("invalid two-factor code")
		}
		if migratedSecret != "" {
			all[i].TOTPSecret = migratedSecret
		}
		all[i].TOTPEnabled = true
		found = true
		break
	}
	if !found {
		return fmt.Errorf("user not found: %s", username)
	}
	if err := users.Save(m.cfg.Admin.UsersFile, all); err != nil {
		return err
	}
	m.RevokeUserSessions(username)
	return nil
}

func (m *Middleware) DisableTOTP(identity *Identity, username string) error {
	username = resolveTargetUsername(identity, username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if err := allowSameUserOrAdmin(identity, username); err != nil {
		return err
	}
	all, err := users.Load(m.cfg.Admin.UsersFile)
	if err != nil {
		return err
	}
	found := false
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
			all[i].TOTPEnabled = false
			all[i].TOTPSecret = ""
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("user not found: %s", username)
	}
	m.RevokeUserSessions(username)
	return users.Save(m.cfg.Admin.UsersFile, all)
}

func allowSameUserOrAdmin(identity *Identity, username string) error {
	if identity == nil {
		return fmt.Errorf("admin identity is required")
	}
	if strings.EqualFold(identity.Username, username) || capabilityAllowed(identity.Capabilities, "users.manage") {
		return nil
	}
	return fmt.Errorf("operation is not allowed for this user")
}

func resolveTargetUsername(identity *Identity, username string) string {
	username = strings.TrimSpace(username)
	if username != "" {
		return username
	}
	if identity == nil {
		return ""
	}
	return identity.Username
}

func sameToken(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
