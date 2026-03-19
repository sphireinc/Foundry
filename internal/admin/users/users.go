package users

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	hashAlgorithm  = "pbkdf2-sha256"
	hashIterations = 120000
	hashKeyLength  = 32
)

type File struct {
	Users []User `yaml:"users"`
}

type User struct {
	Username     string `yaml:"username"`
	Name         string `yaml:"name"`
	Email        string `yaml:"email"`
	Role         string `yaml:"role,omitempty"`
	PasswordHash string `yaml:"password_hash"`
	Disabled     bool   `yaml:"disabled,omitempty"`
}

func Load(path string) ([]User, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file File
	if err := yaml.Unmarshal(b, &file); err != nil {
		return nil, err
	}

	out := make([]User, 0, len(file.Users))
	for _, user := range file.Users {
		user.Username = strings.TrimSpace(user.Username)
		user.Name = strings.TrimSpace(user.Name)
		user.Email = strings.TrimSpace(user.Email)
		user.PasswordHash = strings.TrimSpace(user.PasswordHash)
		if user.Username == "" {
			continue
		}
		out = append(out, user)
	}
	return out, nil
}

func Find(path, username string) (*User, error) {
	entries, err := Load(path)
	if err != nil {
		return nil, err
	}

	username = strings.TrimSpace(strings.ToLower(username))
	for _, user := range entries {
		if strings.ToLower(user.Username) == username {
			return &user, nil
		}
	}
	return nil, os.ErrNotExist
}

func Save(path string, entries []User) error {
	file := File{Users: entries}
	b, err := yaml.Marshal(&file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2SHA256([]byte(password), salt, hashIterations, hashKeyLength)

	return fmt.Sprintf(
		"%s$%d$%s$%s",
		hashAlgorithm,
		hashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encodedHash, password string) bool {
	parts := strings.Split(strings.TrimSpace(encodedHash), "$")
	if len(parts) != 4 || parts[0] != hashAlgorithm {
		return false
	}

	iterations, err := parsePositiveInt(parts[1])
	if err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func parsePositiveInt(raw string) (int, error) {
	var value int
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		value = value*10 + int(r-'0')
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return value, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := 32
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)

	for block := 1; block <= blocks; block++ {
		u := hmacSHA256(password, append(salt, byte(block>>24), byte(block>>16), byte(block>>8), byte(block)))
		t := append([]byte(nil), u...)

		for i := 1; i < iterations; i++ {
			u = hmacSHA256(password, u)
			for j := range t {
				t[j] ^= u[j]
			}
		}

		out = append(out, t...)
	}

	return out[:keyLen]
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}
