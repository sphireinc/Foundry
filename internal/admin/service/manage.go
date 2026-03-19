package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/theme"
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
			Username: user.Username,
			Name:     user.Name,
			Email:    user.Email,
			Role:     user.Role,
			Disabled: user.Disabled,
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

	var passwordHash string
	if strings.TrimSpace(req.Password) != "" {
		passwordHash, err = users.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
	}

	found := false
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
			all[i].Username = username
			all[i].Name = strings.TrimSpace(req.Name)
			all[i].Email = strings.TrimSpace(req.Email)
			all[i].Role = strings.TrimSpace(req.Role)
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
			Role:         strings.TrimSpace(req.Role),
			PasswordHash: passwordHash,
			Disabled:     req.Disabled,
		})
	}

	if err := users.Save(s.cfg.Admin.UsersFile, all); err != nil {
		return nil, err
	}
	return &types.UserSummary{
		Username: username,
		Name:     strings.TrimSpace(req.Name),
		Email:    strings.TrimSpace(req.Email),
		Role:     strings.TrimSpace(req.Role),
		Disabled: req.Disabled,
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
	return users.Save(s.cfg.Admin.UsersFile, out)
}

func (s *Service) LoadConfigDocument(ctx context.Context) (*types.ConfigDocumentResponse, error) {
	_ = ctx
	path := consts.ConfigFilePath
	b, err := s.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &types.ConfigDocumentResponse{Path: path, Raw: string(b)}, nil
}

func (s *Service) SaveConfigDocument(ctx context.Context, raw string) (*types.ConfigDocumentResponse, error) {
	_ = ctx
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("config body is required")
	}
	tmp := &config.Config{}
	if err := config.UnmarshalYAML([]byte(raw), tmp); err != nil {
		return nil, err
	}
	if err := s.fs.MkdirAll(filepath.Dir(consts.ConfigFilePath), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(consts.ConfigFilePath, []byte(raw+"\n"), 0o644); err != nil {
		return nil, err
	}
	return &types.ConfigDocumentResponse{Path: consts.ConfigFilePath, Raw: raw + "\n"}, nil
}

func (s *Service) ListThemes(ctx context.Context) ([]types.ThemeRecord, error) {
	_ = ctx
	items, err := theme.ListInstalled(s.cfg.ThemesDir)
	if err != nil {
		return nil, err
	}
	out := make([]types.ThemeRecord, 0, len(items))
	for _, item := range items {
		if item.Name == "admin-themes" {
			continue
		}
		record := types.ThemeRecord{Name: item.Name, Current: item.Name == s.cfg.Theme}
		if manifest, err := theme.LoadManifest(s.cfg.ThemesDir, item.Name); err == nil {
			record.Title = manifest.Title
			record.Version = manifest.Version
			record.Description = manifest.Description
		}
		record.Valid = theme.ValidateInstalled(s.cfg.ThemesDir, item.Name) == nil
		out = append(out, record)
	}
	return out, nil
}

func (s *Service) SwitchTheme(ctx context.Context, name string) error {
	_ = ctx
	if err := theme.ValidateInstalled(s.cfg.ThemesDir, name); err != nil {
		return err
	}
	if err := theme.SwitchInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}
	s.cfg.Theme = name
	s.invalidateGraphCache()
	return nil
}

func (s *Service) ListPlugins(ctx context.Context) ([]types.PluginRecord, error) {
	_ = ctx
	status, err := s.GetSystemStatus(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]types.PluginRecord, 0, len(status.Plugins))
	for _, plugin := range status.Plugins {
		out = append(out, types.PluginRecord{
			Name:    plugin.Name,
			Title:   plugin.Title,
			Version: plugin.Version,
			Enabled: plugin.Enabled,
			Status:  plugin.Status,
		})
	}
	return out, nil
}

func (s *Service) EnablePlugin(ctx context.Context, name string) error {
	_ = ctx
	if err := plugins.EnableInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}
	if !containsString(s.cfg.Plugins.Enabled, name) {
		s.cfg.Plugins.Enabled = append(s.cfg.Plugins.Enabled, name)
	}
	return nil
}

func (s *Service) DisablePlugin(ctx context.Context, name string) error {
	_ = ctx
	if err := plugins.DisableInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}
	out := make([]string, 0, len(s.cfg.Plugins.Enabled))
	for _, enabled := range s.cfg.Plugins.Enabled {
		if enabled != name {
			out = append(out, enabled)
		}
	}
	s.cfg.Plugins.Enabled = out
	return nil
}

func (s *Service) DeleteMedia(ctx context.Context, reference string) error {
	_ = ctx
	_, fullPath, err := s.resolveMediaItem(reference)
	if err != nil {
		return err
	}
	if err := s.fs.Remove(fullPath); err != nil {
		return err
	}
	if err := s.fs.Remove(mediaMetadataPath(fullPath)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
