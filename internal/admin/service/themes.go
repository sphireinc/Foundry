package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/types"
	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/theme"
)

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
		record := types.ThemeRecord{Name: item.Name, Kind: "frontend", Current: item.Name == s.cfg.Theme}
		if manifest, err := theme.LoadManifest(s.cfg.ThemesDir, item.Name); err == nil {
			record.Title = manifest.Title
			record.Version = manifest.Version
			record.Description = manifest.Description
			record.Repo = manifest.Repo
			record.SDKVersion = manifest.SDKVersion
			record.CompatibilityVersion = manifest.CompatibilityVersion
			record.MinFoundryVersion = manifest.MinFoundryVersion
			record.SupportedLayouts = manifest.RequiredLayouts()
			record.Screenshots = append([]string(nil), manifest.Screenshots...)
			record.ConfigSchema = toFieldSchema(manifest.ConfigSchema)
			record.Security = manifest.Security
			record.SecuritySummary = manifest.Security.Summary()
			if securityReport, secErr := theme.AnalyzeInstalledSecurity(s.cfg.ThemesDir, item.Name); secErr == nil {
				record.SecurityReport = securityReport
			}
		}
		if validation, err := theme.ValidateInstalledDetailed(s.cfg.ThemesDir, item.Name); err == nil {
			record.Valid = validation.Valid
			record.Diagnostics = toValidationDiagnostics(validation.Diagnostics)
		}
		out = append(out, record)
	}
	adminThemes, err := adminui.ListInstalled(s.cfg.ThemesDir)
	if err != nil {
		return nil, err
	}
	for _, item := range adminThemes {
		record := types.ThemeRecord{Name: item.Name, Kind: "admin", Current: item.Name == s.cfg.Admin.Theme}
		if manifest, err := adminui.LoadManifest(s.cfg.ThemesDir, item.Name); err == nil {
			record.Title = manifest.Title
			record.Version = manifest.Version
			record.Description = manifest.Description
			record.Repo = manifest.Repo
			record.AdminAPI = manifest.AdminAPI
			record.SDKVersion = manifest.SDKVersion
			record.CompatibilityVersion = manifest.CompatibilityVersion
			record.Components = append([]string(nil), manifest.Components...)
			record.WidgetSlots = append([]string(nil), manifest.WidgetSlots...)
			record.Screenshots = append([]string(nil), manifest.Screenshots...)
		}
		if validation, err := adminui.ValidateTheme(s.cfg.ThemesDir, item.Name); err == nil {
			record.Valid = validation.Valid
			record.Diagnostics = append(record.Diagnostics, toAdminThemeDiagnostics(validation.Diagnostics)...)
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *Service) InstallTheme(ctx context.Context, url, name, kind string) (*types.ThemeRecord, error) {
	_ = ctx
	installKind := theme.InstallKind(strings.TrimSpace(kind))
	if installKind == "" {
		installKind = theme.InstallKindFrontend
	}
	meta, err := theme.Install(theme.InstallOptions{
		ThemesDir: s.cfg.ThemesDir,
		URL:       url,
		Name:      name,
		Kind:      installKind,
	})
	if err != nil {
		return nil, err
	}

	switch m := meta.(type) {
	case *theme.Manifest:
		record := &types.ThemeRecord{
			Name:                 m.Name,
			Kind:                 "frontend",
			Title:                m.Title,
			Version:              m.Version,
			Description:          m.Description,
			Repo:                 m.Repo,
			SDKVersion:           m.SDKVersion,
			CompatibilityVersion: m.CompatibilityVersion,
			MinFoundryVersion:    m.MinFoundryVersion,
			SupportedLayouts:     append([]string(nil), m.RequiredLayouts()...),
			Screenshots:          append([]string(nil), m.Screenshots...),
			ConfigSchema:         toFieldSchema(m.ConfigSchema),
			Security:             m.Security,
			SecuritySummary:      m.Security.Summary(),
		}
		if securityReport, secErr := theme.AnalyzeInstalledSecurity(s.cfg.ThemesDir, m.Name); secErr == nil {
			record.SecurityReport = securityReport
		}
		if validation, err := theme.ValidateInstalledDetailed(s.cfg.ThemesDir, m.Name); err == nil {
			record.Valid = validation.Valid
			record.Diagnostics = toValidationDiagnostics(validation.Diagnostics)
		}
		return record, nil
	case *adminui.Manifest:
		record := &types.ThemeRecord{
			Name:                 m.Name,
			Kind:                 "admin",
			Title:                m.Title,
			Version:              m.Version,
			Description:          m.Description,
			Repo:                 m.Repo,
			AdminAPI:             m.AdminAPI,
			SDKVersion:           m.SDKVersion,
			CompatibilityVersion: m.CompatibilityVersion,
			Components:           append([]string(nil), m.Components...),
			WidgetSlots:          append([]string(nil), m.WidgetSlots...),
			Screenshots:          append([]string(nil), m.Screenshots...),
		}
		if validation, err := adminui.ValidateTheme(s.cfg.ThemesDir, m.Name); err == nil {
			record.Valid = validation.Valid
			record.Diagnostics = toAdminThemeDiagnostics(validation.Diagnostics)
		}
		return record, nil
	default:
		return nil, fmt.Errorf("unexpected installed theme metadata type")
	}
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

func (s *Service) SwitchAdminTheme(ctx context.Context, name string) error {
	_ = ctx
	validation, err := adminui.ValidateTheme(s.cfg.ThemesDir, name)
	if err != nil {
		return err
	}
	if !validation.Valid {
		return fmt.Errorf("admin theme %q is invalid", name)
	}
	if err := config.UpsertNestedScalar(consts.ConfigFilePath, []string{"admin", "theme"}, name); err != nil {
		return err
	}
	s.cfg.Admin.Theme = name
	return nil
}

func (s *Service) ValidateTheme(ctx context.Context, name, kind string) (*types.ThemeRecord, error) {
	_ = ctx
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if name == "" {
		return nil, fmt.Errorf("theme name is required")
	}
	if kind == "" {
		kind = "frontend"
	}
	items, err := s.ListThemes(context.Background())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name && item.Kind == kind {
			record := item
			return &record, nil
		}
	}
	return nil, fmt.Errorf("theme not found: %s", name)
}
