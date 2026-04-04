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
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

const (
	pbkdf2HashAlgorithm  = "pbkdf2-sha256"
	pbkdf2HashIterations = 120000
	pbkdf2HashKeyLength  = 32

	argon2idHashAlgorithm        = "argon2id"
	argon2idMemoryKB      uint32 = 64 * 1024
	argon2idIterations    uint32 = 3
	argon2idParallelism   uint8  = 2
	argon2idSaltLength           = 16
	argon2idKeyLength     uint32 = 32
)

type File struct {
	Users []User `yaml:"users"`
}

type User struct {
	Username          string    `yaml:"username"`
	Name              string    `yaml:"name"`
	Email             string    `yaml:"email"`
	Role              string    `yaml:"role,omitempty"`
	Capabilities      []string  `yaml:"capabilities,omitempty"`
	PasswordHash      string    `yaml:"password_hash"`
	Disabled          bool      `yaml:"disabled,omitempty"`
	TOTPEnabled       bool      `yaml:"totp_enabled,omitempty"`
	TOTPSecret        string    `yaml:"totp_secret,omitempty"`
	ResetTokenHash    string    `yaml:"reset_token_hash,omitempty"`
	ResetTokenExpires time.Time `yaml:"reset_token_expires,omitempty"`
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
		user.Role = strings.TrimSpace(user.Role)
		user.PasswordHash = strings.TrimSpace(user.PasswordHash)
		user.TOTPSecret = strings.TrimSpace(user.TOTPSecret)
		user.ResetTokenHash = strings.TrimSpace(user.ResetTokenHash)
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

func UpdatePasswordHash(path, username, passwordHash string) error {
	all, err := Load(path)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(strings.ToLower(username))
	for i := range all {
		if strings.ToLower(strings.TrimSpace(all[i].Username)) != username {
			continue
		}
		all[i].PasswordHash = strings.TrimSpace(passwordHash)
		return Save(path, all)
	}
	return os.ErrNotExist
}

func UpdateTOTPSecret(path, username, totpSecret string) error {
	all, err := Load(path)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(strings.ToLower(username))
	for i := range all {
		if strings.ToLower(strings.TrimSpace(all[i].Username)) != username {
			continue
		}
		all[i].TOTPSecret = strings.TrimSpace(totpSecret)
		return Save(path, all)
	}
	return os.ErrNotExist
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	salt := make([]byte, argon2idSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argon2idIterations, argon2idMemoryKB, argon2idParallelism, argon2idKeyLength)
	return fmt.Sprintf(
		"%s$%d$%d$%d$%s$%s",
		argon2idHashAlgorithm,
		argon2idMemoryKB,
		argon2idIterations,
		argon2idParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encodedHash, password string) bool {
	ok, _, err := VerifyPasswordWithUpgrade(encodedHash, password)
	return err == nil && ok
}

func VerifyPasswordWithUpgrade(encodedHash, password string) (bool, string, error) {
	encodedHash = strings.TrimSpace(encodedHash)
	if encodedHash == "" {
		return false, "", nil
	}
	switch hashAlgorithm(encodedHash) {
	case argon2idHashAlgorithm:
		ok, err := verifyArgon2id(encodedHash, password)
		return ok, "", err
	case pbkdf2HashAlgorithm:
		ok, err := verifyPBKDF2(encodedHash, password)
		if err != nil || !ok {
			return ok, "", err
		}
		upgraded, err := HashPassword(password)
		return true, upgraded, err
	default:
		return false, "", nil
	}
}

func hashAlgorithm(encodedHash string) string {
	parts := strings.Split(strings.TrimSpace(encodedHash), "$")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func verifyArgon2id(encodedHash, password string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(encodedHash), "$")
	if len(parts) != 6 || parts[0] != argon2idHashAlgorithm {
		return false, nil
	}
	memory, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return false, err
	}
	iterations, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return false, err
	}
	parallelism, err := strconv.ParseUint(parts[3], 10, 8)
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(want)))
	if len(got) != len(want) {
		return false, nil
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func verifyPBKDF2(encodedHash, password string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(encodedHash), "$")
	if len(parts) != 4 || parts[0] != pbkdf2HashAlgorithm {
		return false, nil
	}

	iterations, err := parsePositiveInt(parts[1])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}

	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	if len(got) != len(want) {
		return false, nil
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
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
