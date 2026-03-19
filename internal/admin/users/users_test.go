package users

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword(hash, "secret-password") {
		t.Fatal("expected password verification to succeed")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("expected wrong password verification to fail")
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
