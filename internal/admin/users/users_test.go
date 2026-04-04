package users

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, "argon2id$") {
		t.Fatalf("expected argon2id hash, got %q", hash)
	}
	if !VerifyPassword(hash, "secret-password") {
		t.Fatal("expected password verification to succeed")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("expected wrong password verification to fail")
	}
}

func TestVerifyPasswordWithUpgradeFromPBKDF2(t *testing.T) {
	legacySalt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	legacyKey := base64.RawStdEncoding.EncodeToString(pbkdf2SHA256([]byte("secret-password"), []byte("0123456789abcdef"), pbkdf2HashIterations, pbkdf2HashKeyLength))
	legacy := pbkdf2HashAlgorithm + "$120000$" + legacySalt + "$" + legacyKey

	ok, upgraded, err := VerifyPasswordWithUpgrade(legacy, "secret-password")
	if err != nil {
		t.Fatalf("verify legacy password: %v", err)
	}
	if !ok {
		t.Fatal("expected legacy password verification to succeed")
	}
	if !strings.HasPrefix(upgraded, "argon2id$") {
		t.Fatalf("expected upgraded argon2id hash, got %q", upgraded)
	}
	if !VerifyPassword(upgraded, "secret-password") {
		t.Fatal("expected upgraded hash to verify")
	}
}

func TestLoadAndFindUsers(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "admin-users.yaml")
	hash, err := HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	body := []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    password_hash: " + hash + "\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("load users: %v", err)
	}
	if len(entries) != 1 || entries[0].Username != "admin" {
		t.Fatalf("unexpected loaded users: %#v", entries)
	}

	user, err := Find(path, "admin")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if user.Email != "admin@example.com" {
		t.Fatalf("unexpected user: %#v", user)
	}
}
