package mcp

import (
	"encoding/json"
	"strings"
)

const Redacted = "[redacted]"

var sensitiveKeys = map[string]struct{}{
	"access_token":      {},
	"api_key":           {},
	"apikey":            {},
	"client_secret":     {},
	"connection_string": {},
	"database_url":      {},
	"db_dsn":            {},
	"dsn":               {},
	"passwd":            {},
	"password":          {},
	"password_hash":     {},
	"private_key":       {},
	"secret":            {},
	"session_secret":    {},
	"shared_secret":     {},
	"signing_key":       {},
	"token":             {},
	"totp_secret":       {},
	"totp_secret_key":   {},
}

var sensitiveFragments = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"private_key",
	"credential",
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	if _, ok := sensitiveKeys[normalized]; ok {
		return true
	}
	for _, fragment := range sensitiveFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func Redact(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, inner := range typed {
			if isSensitiveKey(key) {
				out[key] = Redacted
				continue
			}
			out[key] = Redact(inner)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, inner := range typed {
			out[i] = Redact(inner)
		}
		return out
	default:
		return value
	}
}

func RedactJSON(value any) any {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return Redacted
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return Redacted
	}
	return Redact(decoded)
}
