package auth

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/sphireinc/foundry/internal/config"
)

func ValidatePassword(cfg *config.Config, password string) error {
	password = strings.TrimSpace(password)
	minLength := 12
	if cfg != nil && cfg.Admin.PasswordMinLength > 0 {
		minLength = cfg.Admin.PasswordMinLength
	}
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters", minLength)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password must include upper, lower, number, and special characters")
	}
	return nil
}
