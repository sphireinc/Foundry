package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const totpSecretPrefix = "enc:v1:"

func (m *Middleware) requireTOTPSecretKey() ([]byte, error) {
	if m == nil || m.cfg == nil {
		return nil, fmt.Errorf("admin auth is not configured")
	}
	raw := strings.TrimSpace(m.cfg.Admin.TOTPSecretKey)
	if raw == "" {
		return nil, fmt.Errorf("TOTP secret encryption key is not configured")
	}
	key, err := decodeBase64Key(raw)
	if err != nil {
		return nil, fmt.Errorf("decode TOTP secret key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("TOTP secret encryption key must decode to 32 bytes")
	}
	return key, nil
}

func (m *Middleware) encryptTOTPSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	key, err := m.requireTOTPSecretKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return totpSecretPrefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func (m *Middleware) decodeTOTPSecret(stored string) (plain string, migrated string, err error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", "", nil
	}
	if !strings.HasPrefix(stored, totpSecretPrefix) {
		if strings.TrimSpace(m.cfg.Admin.TOTPSecretKey) != "" {
			migrated, err = m.encryptTOTPSecret(stored)
			if err != nil {
				return "", "", err
			}
		}
		return stored, migrated, nil
	}
	key, err := m.requireTOTPSecretKey()
	if err != nil {
		return "", "", err
	}
	body, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, totpSecretPrefix))
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	if len(body) < gcm.NonceSize() {
		return "", "", fmt.Errorf("invalid encrypted TOTP secret")
	}
	nonce := body[:gcm.NonceSize()]
	ciphertext := body[gcm.NonceSize():]
	plainBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(plainBytes)), "", nil
}

func decodeBase64Key(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	if key, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return key, nil
	}
	return base64.StdEncoding.DecodeString(raw)
}
