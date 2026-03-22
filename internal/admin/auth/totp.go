package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

func VerifyTOTP(secret, code string, now time.Time) bool {
	secret = strings.TrimSpace(secret)
	code = normalizeTOTPCode(code)
	if secret == "" || code == "" {
		return false
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	for offset := -1; offset <= 1; offset++ {
		if totpCodeForCounter(key, counterForTime(now.Add(time.Duration(offset)*30*time.Second))) == code {
			return true
		}
	}
	return false
}

func TOTPProvisioningURI(issuer, username, secret string) string {
	issuer = strings.TrimSpace(issuer)
	username = strings.TrimSpace(username)
	issuer = firstNonEmptyString(issuer, "Foundry")
	label := url.QueryEscape(issuer) + ":" + url.QueryEscape(username)
	return "otpauth://totp/" + label + "?secret=" + url.QueryEscape(secret) + "&issuer=" + url.QueryEscape(issuer)
}

func normalizeTOTPCode(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	if len(value) != 6 {
		return ""
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value
}

func counterForTime(now time.Time) uint64 {
	return uint64(now.UTC().Unix() / 30)
}

func totpCodeForCounter(key []byte, counter uint64) string {
	msg := []byte{
		byte(counter >> 56), byte(counter >> 48), byte(counter >> 40), byte(counter >> 32),
		byte(counter >> 24), byte(counter >> 16), byte(counter >> 8), byte(counter),
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binary := (int(sum[offset])&0x7f)<<24 | int(sum[offset+1])<<16 | int(sum[offset+2])<<8 | int(sum[offset+3])
	return fmt.Sprintf("%06s", strconv.Itoa(binary%1000000))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
