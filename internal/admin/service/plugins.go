package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/plugins"
)

func (s *Service) ListPlugins(ctx context.Context) ([]types.PluginRecord, error) {
	_ = ctx
	status, err := s.GetSystemStatus(context.Background())
	if err != nil {
		return nil, err
	}
	installed, err := plugins.ListInstalled(s.cfg.PluginsDir)
	if err != nil {
		return nil, err
	}
	metaByName := make(map[string]plugins.Metadata, len(installed))
	for _, meta := range installed {
		metaByName[meta.Name] = meta
	}
	out := make([]types.PluginRecord, 0, len(status.Plugins))
	for _, pluginStatus := range status.Plugins {
		record := types.PluginRecord{
			Name:    pluginStatus.Name,
			Title:   pluginStatus.Title,
			Version: pluginStatus.Version,
			Enabled: pluginStatus.Enabled,
			Status:  pluginStatus.Status,
			Health:  pluginStatus.Status,
		}
		if meta, ok := metaByName[pluginStatus.Name]; ok {
			record.Description = meta.Description
			record.Author = meta.Author
			record.Repo = meta.Repo
			record.CompatibilityVersion = meta.CompatibilityVersion
			record.MinFoundryVersion = meta.MinFoundryVersion
			record.FoundryAPI = meta.FoundryAPI
			record.Requires = append([]string(nil), meta.Requires...)
			record.ConfigSchema = toFieldSchema(meta.ConfigSchema)
			record.Dependencies = toPluginDependencies(meta.Dependencies)
			record.Permissions = meta.Permissions
			record.RiskTier = meta.Permissions.RiskTier()
			record.RequiresApproval = meta.Permissions.Capabilities.RequiresAdminApproval
			record.PermissionSummary = meta.Permissions.Summary()
			record.Runtime = meta.Runtime
			record.RuntimeSummary = meta.Runtime.Summary()
			if hasRollback, _ := plugins.HasRollback(s.cfg.PluginsDir, meta.Name); hasRollback {
				record.CanRollback = true
			}
			report := plugins.DiagnoseInstalled(s.cfg.PluginsDir, meta, pluginStatus.Enabled)
			record.Health = report.Status
			record.Diagnostics = toPluginDiagnostics(report.Diagnostics)
			record.RiskTier = report.Security.RiskTier
			record.RequiresApproval = report.Security.RequiresApproval
			security := report.Security
			record.Security = &security
			record.SecurityFindings = append([]plugins.SecurityFinding(nil), report.Security.Findings...)
			record.SecurityMismatches = toPluginDiagnostics(report.Security.Mismatches)
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *Service) ValidatePlugin(ctx context.Context, name string) (*types.PluginRecord, error) {
	_ = ctx
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	items, err := s.ListPlugins(context.Background())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name {
			record := item
			return &record, nil
		}
	}
	return nil, fmt.Errorf("plugin not found: %s", name)
}

func (s *Service) EnablePlugin(ctx context.Context, name string, approveRisk, acknowledgeMismatches bool) error {
	_ = ctx
	meta, err := plugins.LoadMetadata(s.cfg.PluginsDir, name)
	if err != nil {
		return err
	}
	report := plugins.AnalyzeInstalled(meta)
	if plugins.SecurityApprovalRequired(meta, report) && !approveRisk {
		return fmt.Errorf("plugin %q requires explicit risk approval before enabling", name)
	}
	if len(report.Mismatches) > 0 && !acknowledgeMismatches {
		return fmt.Errorf("plugin %q has %d security mismatch(es); acknowledge_mismatches is required", name, len(report.Mismatches))
	}
	if err := plugins.EnsureRuntimeSupported(meta); err != nil {
		return err
	}
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

func (s *Service) InstallPlugin(ctx context.Context, url, name string, approveRisk, acknowledgeMismatches bool) (*types.PluginRecord, error) {
	_ = ctx
	meta, err := plugins.Install(plugins.InstallOptions{
		PluginsDir:  s.cfg.PluginsDir,
		URL:         url,
		Name:        name,
		ApproveRisk: approveRisk,
	})
	if err != nil {
		return nil, err
	}
	report := plugins.DiagnoseInstalled(s.cfg.PluginsDir, meta, false)
	if len(report.Security.Mismatches) > 0 && !acknowledgeMismatches {
		_ = plugins.Uninstall(s.cfg.PluginsDir, meta.Name)
		return nil, fmt.Errorf("plugin %q has %d security mismatch(es); acknowledge_mismatches is required", meta.Name, len(report.Security.Mismatches))
	}
	security := report.Security
	return &types.PluginRecord{
		Name:                 meta.Name,
		Title:                meta.Title,
		Version:              meta.Version,
		Description:          meta.Description,
		Author:               meta.Author,
		Repo:                 meta.Repo,
		Status:               "installed",
		Health:               report.Status,
		FoundryAPI:           meta.FoundryAPI,
		MinFoundryVersion:    meta.MinFoundryVersion,
		CompatibilityVersion: meta.CompatibilityVersion,
		Requires:             append([]string(nil), meta.Requires...),
		Dependencies:         toPluginDependencies(meta.Dependencies),
		ConfigSchema:         toFieldSchema(meta.ConfigSchema),
		Permissions:          meta.Permissions,
		RiskTier:             report.Security.RiskTier,
		RequiresApproval:     report.Security.RequiresApproval,
		PermissionSummary:    meta.Permissions.Summary(),
		Runtime:              meta.Runtime,
		RuntimeSummary:       meta.Runtime.Summary(),
		Security:             &security,
		SecurityFindings:     append([]plugins.SecurityFinding(nil), report.Security.Findings...),
		SecurityMismatches:   toPluginDiagnostics(report.Security.Mismatches),
	}, nil
}

func (s *Service) UpdatePlugin(ctx context.Context, name string, approveRisk, acknowledgeMismatches bool) (*types.PluginRecord, error) {
	_ = ctx
	meta, err := plugins.UpdateInstalled(s.cfg.PluginsDir, name, approveRisk)
	if err != nil {
		return nil, err
	}
	report := plugins.DiagnoseInstalled(s.cfg.PluginsDir, meta, containsString(s.cfg.Plugins.Enabled, meta.Name))
	if len(report.Security.Mismatches) > 0 && !acknowledgeMismatches {
		return nil, fmt.Errorf("plugin %q has %d security mismatch(es); acknowledge_mismatches is required", meta.Name, len(report.Security.Mismatches))
	}
	security := report.Security
	return &types.PluginRecord{
		Name:                 meta.Name,
		Title:                meta.Title,
		Version:              meta.Version,
		Description:          meta.Description,
		Author:               meta.Author,
		Repo:                 meta.Repo,
		Enabled:              containsString(s.cfg.Plugins.Enabled, meta.Name),
		Status:               "updated",
		Health:               report.Status,
		CanRollback:          true,
		FoundryAPI:           meta.FoundryAPI,
		MinFoundryVersion:    meta.MinFoundryVersion,
		CompatibilityVersion: meta.CompatibilityVersion,
		Requires:             append([]string(nil), meta.Requires...),
		Dependencies:         toPluginDependencies(meta.Dependencies),
		ConfigSchema:         toFieldSchema(meta.ConfigSchema),
		Permissions:          meta.Permissions,
		RiskTier:             report.Security.RiskTier,
		RequiresApproval:     report.Security.RequiresApproval,
		PermissionSummary:    meta.Permissions.Summary(),
		Runtime:              meta.Runtime,
		RuntimeSummary:       meta.Runtime.Summary(),
		Security:             &security,
		SecurityFindings:     append([]plugins.SecurityFinding(nil), report.Security.Findings...),
		SecurityMismatches:   toPluginDiagnostics(report.Security.Mismatches),
		Diagnostics:          toPluginDiagnostics(report.Diagnostics),
	}, nil
}

func (s *Service) RollbackPlugin(ctx context.Context, name string) (*types.PluginRecord, error) {
	_ = ctx
	meta, err := plugins.RollbackInstalled(s.cfg.PluginsDir, name)
	if err != nil {
		return nil, err
	}
	report := plugins.DiagnoseInstalled(s.cfg.PluginsDir, meta, containsString(s.cfg.Plugins.Enabled, meta.Name))
	security := report.Security
	return &types.PluginRecord{
		Name:                 meta.Name,
		Title:                meta.Title,
		Version:              meta.Version,
		Description:          meta.Description,
		Author:               meta.Author,
		Repo:                 meta.Repo,
		Enabled:              containsString(s.cfg.Plugins.Enabled, meta.Name),
		Status:               "rolled back",
		Health:               report.Status,
		CanRollback:          true,
		FoundryAPI:           meta.FoundryAPI,
		MinFoundryVersion:    meta.MinFoundryVersion,
		CompatibilityVersion: meta.CompatibilityVersion,
		Requires:             append([]string(nil), meta.Requires...),
		Dependencies:         toPluginDependencies(meta.Dependencies),
		ConfigSchema:         toFieldSchema(meta.ConfigSchema),
		Permissions:          meta.Permissions,
		RiskTier:             report.Security.RiskTier,
		RequiresApproval:     report.Security.RequiresApproval,
		PermissionSummary:    meta.Permissions.Summary(),
		Runtime:              meta.Runtime,
		RuntimeSummary:       meta.Runtime.Summary(),
		Security:             &security,
		SecurityFindings:     append([]plugins.SecurityFinding(nil), report.Security.Findings...),
		SecurityMismatches:   toPluginDiagnostics(report.Security.Mismatches),
		Diagnostics:          toPluginDiagnostics(report.Diagnostics),
	}, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
