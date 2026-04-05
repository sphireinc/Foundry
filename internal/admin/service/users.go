package service

import (
	"context"
	"fmt"
	"strings"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
)

func (s *Service) ListUsers(ctx context.Context) ([]types.UserSummary, error) {
	_ = ctx
	list, err := users.Load(s.cfg.Admin.UsersFile)
	if err != nil {
		return nil, err
	}
	out := make([]types.UserSummary, 0, len(list))
	for _, user := range list {
		out = append(out, types.UserSummary{
			Username:     user.Username,
			Name:         user.Name,
			Email:        user.Email,
			Role:         normalizeUserRole(user.Role),
			Capabilities: append([]string(nil), user.Capabilities...),
			Disabled:     user.Disabled,
			TOTPEnabled:  user.TOTPEnabled,
		})
	}
	return out, nil
}

func (s *Service) SaveUser(ctx context.Context, req types.UserSaveRequest) (*types.UserSummary, error) {
	_ = ctx
	all, err := users.Load(s.cfg.Admin.UsersFile)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	role := normalizeUserRole(req.Role)

	var passwordHash string
	if strings.TrimSpace(req.Password) != "" {
		if err := adminauth.ValidatePassword(s.cfg, req.Password); err != nil {
			return nil, err
		}
		passwordHash, err = users.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
	}

	found := false
	revokeSessions := false
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
			if passwordHash != "" {
				revokeSessions = true
			}
			if all[i].Disabled != req.Disabled {
				revokeSessions = true
			}
			if strings.TrimSpace(all[i].Role) != role {
				revokeSessions = true
			}
			if !stringSlicesEqual(all[i].Capabilities, req.Capabilities) {
				revokeSessions = true
			}
			all[i].Username = username
			all[i].Name = strings.TrimSpace(req.Name)
			all[i].Email = strings.TrimSpace(req.Email)
			all[i].Role = role
			all[i].Capabilities = append([]string(nil), req.Capabilities...)
			all[i].Disabled = req.Disabled
			if passwordHash != "" {
				all[i].PasswordHash = passwordHash
			}
			found = true
			break
		}
	}
	if !found {
		if passwordHash == "" {
			return nil, fmt.Errorf("password is required for a new user")
		}
		all = append(all, users.User{
			Username:     username,
			Name:         strings.TrimSpace(req.Name),
			Email:        strings.TrimSpace(req.Email),
			Role:         role,
			Capabilities: append([]string(nil), req.Capabilities...),
			PasswordHash: passwordHash,
			Disabled:     req.Disabled,
		})
	}

	if err := users.Save(s.cfg.Admin.UsersFile, all); err != nil {
		return nil, err
	}
	if revokeSessions {
		adminauth.New(s.cfg).RevokeUserSessions(username)
	}
	return &types.UserSummary{
		Username:     username,
		Name:         strings.TrimSpace(req.Name),
		Email:        strings.TrimSpace(req.Email),
		Role:         role,
		Capabilities: append([]string(nil), req.Capabilities...),
		Disabled:     req.Disabled,
	}, nil
}

func (s *Service) DeleteUser(ctx context.Context, username string) error {
	_ = ctx
	all, err := users.Load(s.cfg.Admin.UsersFile)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	out := make([]users.User, 0, len(all))
	removed := false
	for _, user := range all {
		if strings.EqualFold(user.Username, username) {
			removed = true
			continue
		}
		out = append(out, user)
	}
	if !removed {
		return fmt.Errorf("user not found: %s", username)
	}
	if err := users.Save(s.cfg.Admin.UsersFile, out); err != nil {
		return err
	}
	adminauth.New(s.cfg).RevokeUserSessions(username)
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

func normalizeUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "editor":
		return "editor"
	case "author":
		return "author"
	case "reviewer":
		return "reviewer"
	default:
		return "editor"
	}
}
