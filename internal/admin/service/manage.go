package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/types"
	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/backup"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/customfields"
	"github.com/sphireinc/foundry/internal/hostservice"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/standalone"
	"github.com/sphireinc/foundry/internal/theme"
	"github.com/sphireinc/foundry/internal/updater"
	"gopkg.in/yaml.v3"
)

func (s *Service) ListBackups(ctx context.Context) ([]types.BackupRecord, error) {
	_ = ctx
	items, err := backup.List(s.cfg.Backup.Dir)
	if err != nil {
		return nil, err
	}
	out := make([]types.BackupRecord, 0, len(items))
	for _, item := range items {
		out = append(out, backupRecord(item))
	}
	return out, nil
}

func (s *Service) CreateBackup(ctx context.Context, name string) (*types.BackupRecord, error) {
	_ = ctx
	name = strings.TrimSpace(name)
	var (
		snapshot *backup.Snapshot
		err      error
	)
	if name == "" {
		snapshot, err = backup.CreateManagedSnapshot(s.cfg)
	} else {
		target := filepath.Join(s.cfg.Backup.Dir, filepath.Base(name))
		snapshot, err = backup.CreateZipSnapshot(s.cfg, target)
	}
	if err != nil {
		return nil, err
	}
	record := backupRecord(*snapshot)
	return &record, nil
}

func (s *Service) RestoreBackup(ctx context.Context, name string) (*types.BackupRecord, error) {
	_ = ctx
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" {
		return nil, fmt.Errorf("backup name is required")
	}
	target := filepath.Join(s.cfg.Backup.Dir, name)
	if err := backup.RestoreZipSnapshot(s.cfg, target); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	return &types.BackupRecord{
		Name:      filepath.Base(target),
		Path:      target,
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) BackupPath(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" {
		return "", fmt.Errorf("backup name is required")
	}
	target := filepath.Join(s.cfg.Backup.Dir, name)
	if !backup.PathIsUnderBackupRoot(s.cfg, target) {
		return "", fmt.Errorf("backup path is outside backup root")
	}
	if _, err := os.Stat(target); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Service) CreateGitBackupSnapshot(ctx context.Context, message string, push bool) (*types.BackupGitSnapshotRecord, error) {
	_ = ctx
	snapshot, err := backup.CreateGitSnapshot(s.cfg, message, push)
	if err != nil {
		return nil, err
	}
	return &types.BackupGitSnapshotRecord{
		RepoDir:   snapshot.RepoDir,
		Revision:  snapshot.Revision,
		CreatedAt: snapshot.CreatedAt.UTC().Format(time.RFC3339),
		Message:   snapshot.Message,
		Changed:   snapshot.Changed,
		Pushed:    snapshot.Pushed,
		RemoteURL: snapshot.RemoteURL,
		Branch:    snapshot.Branch,
	}, nil
}

func (s *Service) ListGitBackupSnapshots(ctx context.Context, limit int) ([]types.BackupGitSnapshotRecord, error) {
	_ = ctx
	items, err := backup.ListGitSnapshots(s.cfg, limit)
	if err != nil {
		return nil, err
	}
	out := make([]types.BackupGitSnapshotRecord, 0, len(items))
	for _, item := range items {
		out = append(out, types.BackupGitSnapshotRecord{
			RepoDir:   item.RepoDir,
			Revision:  item.Revision,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
			Message:   item.Message,
			Changed:   item.Changed,
			Pushed:    item.Pushed,
			RemoteURL: item.RemoteURL,
			Branch:    item.Branch,
		})
	}
	return out, nil
}

func (s *Service) GetOperationsStatus(ctx context.Context) (*types.OperationsStatusResponse, error) {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	resp := &types.OperationsStatusResponse{}
	serviceStatus, err := hostservice.CheckStatus(projectDir)
	if err == nil && serviceStatus != nil {
		resp.ServiceInstalled = serviceStatus.Installed
		resp.ServiceRunning = serviceStatus.Running
		resp.ServiceEnabled = serviceStatus.Enabled
		resp.ServiceMessage = serviceStatus.Message
		if serviceStatus.Metadata != nil {
			resp.ServiceName = serviceStatus.Metadata.Name
			resp.ServiceFile = serviceStatus.Metadata.ServicePath
			resp.ServiceLog = serviceStatus.Metadata.LogPath
		}
	}
	if standaloneState, running, err := standalone.RunningState(projectDir); err == nil && standaloneState != nil {
		resp.StandalonePID = standaloneState.PID
		resp.StandaloneLog = standaloneState.LogPath
		resp.StandaloneActive = running
	}
	status, err := s.GetSystemStatus(context.Background())
	if err == nil && status != nil {
		resp.Checks = append([]types.HealthCheck(nil), status.Checks...)
	}
	return resp, nil
}

func (s *Service) ReadOperationsLog(ctx context.Context, lines int) (*types.OperationsLogResponse, error) {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	serviceStatus, err := hostservice.CheckStatus(projectDir)
	if err == nil && serviceStatus != nil && serviceStatus.Metadata != nil && strings.TrimSpace(serviceStatus.Metadata.LogPath) != "" {
		content, readErr := standalone.ReadLastLines(serviceStatus.Metadata.LogPath, lines)
		if readErr == nil {
			return &types.OperationsLogResponse{
				Source:  "service",
				LogPath: serviceStatus.Metadata.LogPath,
				Content: content,
			}, nil
		}
	}
	if standaloneState, running, err := standalone.RunningState(projectDir); err == nil && standaloneState != nil && running {
		content, readErr := standalone.ReadLastLines(standaloneState.LogPath, lines)
		if readErr == nil {
			return &types.OperationsLogResponse{
				Source:  "standalone",
				LogPath: standaloneState.LogPath,
				Content: content,
			}, nil
		}
	}
	return &types.OperationsLogResponse{Source: "none", Content: ""}, nil
}

func (s *Service) ClearOperationalCaches(ctx context.Context) error {
	_ = ctx
	s.invalidateGraphCache()
	return nil
}

func (s *Service) RebuildSite(ctx context.Context) error {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	executable, err := hostservice.EnsureExecutable(projectDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}
	s.invalidateGraphCache()
	return nil
}

func backupRecord(item backup.Snapshot) types.BackupRecord {
	return types.BackupRecord{
		Name:      filepath.Base(item.Path),
		Path:      item.Path,
		SizeBytes: item.SizeBytes,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) CheckForUpdates(ctx context.Context) (*types.UpdateStatusResponse, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	logx.Info("admin update check requested", "project_dir", projectDir)
	info, err := updater.Check(ctx, projectDir)
	if err != nil {
		return nil, err
	}
	logx.Info("admin update check completed", "current_version", info.CurrentVersion, "latest_version", info.LatestVersion, "has_update", info.HasUpdate, "install_mode", info.InstallMode, "apply_supported", info.ApplySupported)
	return &types.UpdateStatusResponse{
		Repo:                  info.Repo,
		CurrentVersion:        info.CurrentVersion,
		CurrentDisplayVersion: info.CurrentDisplayVersion,
		LatestVersion:         info.LatestVersion,
		HasUpdate:             info.HasUpdate,
		InstallMode:           string(info.InstallMode),
		ApplySupported:        info.ApplySupported,
		ReleaseURL:            info.ReleaseURL,
		PublishedAt:           info.PublishedAt.UTC().Format(time.RFC3339),
		Body:                  info.Body,
		AssetName:             info.AssetName,
		Instructions:          info.Instructions,
		NearestTag:            info.NearestTag,
		CurrentCommit:         info.CurrentCommit,
		Dirty:                 info.Dirty,
	}, nil
}

func (s *Service) ApplyUpdate(ctx context.Context) (*types.UpdateStatusResponse, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	logx.Info("admin update apply requested", "project_dir", projectDir)
	info, err := updater.ScheduleApply(ctx, projectDir)
	if err != nil {
		return nil, err
	}
	logx.Info("admin update apply scheduled", "current_version", info.CurrentVersion, "latest_version", info.LatestVersion, "install_mode", info.InstallMode, "asset_name", info.AssetName)
	return &types.UpdateStatusResponse{
		Repo:                  info.Repo,
		CurrentVersion:        info.CurrentVersion,
		CurrentDisplayVersion: info.CurrentDisplayVersion,
		LatestVersion:         info.LatestVersion,
		HasUpdate:             info.HasUpdate,
		InstallMode:           string(info.InstallMode),
		ApplySupported:        info.ApplySupported,
		ReleaseURL:            info.ReleaseURL,
		PublishedAt:           info.PublishedAt.UTC().Format(time.RFC3339),
		Body:                  info.Body,
		AssetName:             info.AssetName,
		Instructions:          info.Instructions,
		NearestTag:            info.NearestTag,
		CurrentCommit:         info.CurrentCommit,
		Dirty:                 info.Dirty,
	}, nil
}

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

func (s *Service) ListSettingsSections(ctx context.Context) ([]types.SettingsSection, error) {
	_ = ctx
	sections := []types.SettingsSection{
		{Key: "site", Title: "Site", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "server", Title: "Server", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "build", Title: "Build", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "content", Title: "Content", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "taxonomies", Title: "Taxonomies", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "plugins", Title: "Plugins", Capability: "plugins.manage", Writable: true, Source: "core"},
		{Key: "seo", Title: "SEO", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "feed", Title: "Feed", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "deploy", Title: "Deploy", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "params", Title: "Params", Capability: "config.manage", Writable: true, Source: "core"},
		{Key: "menus", Title: "Menus", Capability: "config.manage", Writable: true, Source: "core"},
	}

	for pluginName, meta := range s.pluginMetadata() {
		for _, section := range meta.AdminExtensions.SettingsSections {
			sections = append(sections, types.SettingsSection{
				Key:         section.Key,
				Title:       section.Title,
				Capability:  firstNonEmptyString(section.Capability, "config.manage"),
				Description: section.Description,
				Writable:    true,
				Source:      pluginName,
				Schema:      toFieldSchema(section.Schema),
			})
		}
	}

	return sections, nil
}

func (s *Service) ListAdminExtensions(ctx context.Context) (*types.AdminExtensionRegistry, error) {
	_ = ctx
	registry := &types.AdminExtensionRegistry{}
	for pluginName, meta := range s.pluginMetadata() {
		for _, page := range meta.AdminExtensions.Pages {
			moduleURL, styleURLs := s.adminExtensionAssetURLs(pluginName, page.Module, page.Styles)
			registry.Pages = append(registry.Pages, types.AdminExtensionPage{
				Plugin:      pluginName,
				Key:         page.Key,
				Title:       page.Title,
				Route:       page.Route,
				NavGroup:    page.NavGroup,
				Capability:  page.Capability,
				Description: page.Description,
				ModuleURL:   moduleURL,
				StyleURLs:   styleURLs,
			})
		}
		for _, widget := range meta.AdminExtensions.Widgets {
			moduleURL, styleURLs := s.adminExtensionAssetURLs(pluginName, widget.Module, widget.Styles)
			registry.Widgets = append(registry.Widgets, types.AdminExtensionWidget{
				Plugin:      pluginName,
				Key:         widget.Key,
				Title:       widget.Title,
				Slot:        widget.Slot,
				Capability:  widget.Capability,
				Description: widget.Description,
				ModuleURL:   moduleURL,
				StyleURLs:   styleURLs,
			})
		}
		for _, slot := range meta.AdminExtensions.Slots {
			registry.Slots = append(registry.Slots, types.AdminExtensionSlot{
				Plugin:      pluginName,
				Name:        slot.Name,
				Description: slot.Description,
			})
		}
		for _, setting := range meta.AdminExtensions.SettingsSections {
			registry.Settings = append(registry.Settings, types.AdminExtensionSetting{
				Plugin:      pluginName,
				Key:         setting.Key,
				Title:       setting.Title,
				Capability:  setting.Capability,
				Description: setting.Description,
				Schema:      toFieldSchema(setting.Schema),
			})
		}
	}
	return registry, nil
}

func (s *Service) AllowsAdminAsset(pluginName, assetPath string) bool {
	pluginName = strings.TrimSpace(pluginName)
	assetPath = strings.TrimSpace(assetPath)
	if pluginName == "" || assetPath == "" {
		return false
	}

	meta, ok := s.pluginMetadata()[pluginName]
	if !ok {
		return false
	}

	allowed := make(map[string]struct{})
	for _, page := range meta.AdminExtensions.Pages {
		if clean, err := plugins.NormalizeAdminAssetPath(page.Module); err == nil && clean != "" {
			allowed[clean] = struct{}{}
		}
		for _, style := range page.Styles {
			if clean, err := plugins.NormalizeAdminAssetPath(style); err == nil && clean != "" {
				allowed[clean] = struct{}{}
			}
		}
	}
	for _, widget := range meta.AdminExtensions.Widgets {
		if clean, err := plugins.NormalizeAdminAssetPath(widget.Module); err == nil && clean != "" {
			allowed[clean] = struct{}{}
		}
		for _, style := range widget.Styles {
			if clean, err := plugins.NormalizeAdminAssetPath(style); err == nil && clean != "" {
				allowed[clean] = struct{}{}
			}
		}
	}

	_, ok = allowed[assetPath]
	return ok
}

func (s *Service) adminExtensionAssetURLs(pluginName, module string, styles []string) (string, []string) {
	var moduleURL string
	if clean, err := plugins.NormalizeAdminAssetPath(module); err == nil && clean != "" {
		moduleURL = s.cfg.AdminPath() + "/extensions/" + pluginName + "/" + clean
	}

	styleURLs := make([]string, 0, len(styles))
	for _, style := range styles {
		clean, err := plugins.NormalizeAdminAssetPath(style)
		if err != nil || clean == "" {
			continue
		}
		styleURLs = append(styleURLs, s.cfg.AdminPath()+"/extensions/"+pluginName+"/"+clean)
	}

	return moduleURL, styleURLs
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
	for i := range all {
		if strings.EqualFold(all[i].Username, username) {
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
	return &types.UserSummary{
		Username:     username,
		Name:         strings.TrimSpace(req.Name),
		Email:        strings.TrimSpace(req.Email),
		Role:         role,
		Capabilities: append([]string(nil), req.Capabilities...),
		Disabled:     req.Disabled,
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func (s *Service) LoadCustomCSSDocument(ctx context.Context) (*types.CustomCSSDocumentResponse, error) {
	_ = ctx
	path := filepath.Join(s.cfg.ContentDir, s.cfg.Content.AssetsDir, "css", "custom.css")
	b, err := s.fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.CustomCSSDocumentResponse{Path: path, Raw: ""}, nil
		}
		return nil, err
	}
	return &types.CustomCSSDocumentResponse{Path: path, Raw: string(b)}, nil
}

func (s *Service) SaveCustomCSSDocument(ctx context.Context, raw string) (*types.CustomCSSDocumentResponse, error) {
	_ = ctx
	path := filepath.Join(s.cfg.ContentDir, s.cfg.Content.AssetsDir, "css", "custom.css")
	if err := s.fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if normalized != "" && !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	if err := s.fs.WriteFile(path, []byte(normalized), 0o644); err != nil {
		return nil, err
	}
	return &types.CustomCSSDocumentResponse{Path: path, Raw: normalized}, nil
}

func (s *Service) LoadCustomFieldsDocument(ctx context.Context) (*types.CustomFieldsDocumentResponse, error) {
	_ = ctx
	store, err := customfields.Load(s.cfg)
	if err != nil {
		return nil, err
	}
	path := customfields.Path(s.cfg)
	raw, err := s.fs.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &types.CustomFieldsDocumentResponse{
		Path:      displayCustomFieldsPath(path),
		Raw:       string(raw),
		Values:    customfields.NormalizeValues(store.Values),
		Contracts: sharedFieldContractsForManifest(s.activeThemeManifest()),
	}, nil
}

func (s *Service) SaveCustomFieldsDocument(ctx context.Context, raw string, values map[string]any) (*types.CustomFieldsDocumentResponse, error) {
	_ = ctx
	path := customfields.Path(s.cfg)
	var store customfields.Store
	if strings.TrimSpace(raw) != "" {
		if err := yaml.Unmarshal([]byte(raw), &store); err != nil {
			return nil, err
		}
	} else {
		store.Values = customfields.NormalizeValues(values)
	}
	store.Values = customfields.NormalizeValues(store.Values)
	if err := customfields.ValidateRoot(store.Values); err != nil {
		return nil, err
	}
	body, err := yaml.Marshal(&store)
	if err != nil {
		return nil, err
	}
	if err := s.fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	return &types.CustomFieldsDocumentResponse{
		Path:      displayCustomFieldsPath(path),
		Raw:       string(body),
		Values:    customfields.NormalizeValues(store.Values),
		Contracts: sharedFieldContractsForManifest(s.activeThemeManifest()),
	}, nil
}

func (s *Service) LoadSettingsForm(ctx context.Context) (*types.SettingsFormResponse, error) {
	_ = ctx
	body, err := s.fs.ReadFile(consts.ConfigFilePath)
	if err != nil {
		return nil, err
	}
	var cfg config.Config
	if err := config.UnmarshalYAML(body, &cfg); err != nil {
		return nil, err
	}
	return &types.SettingsFormResponse{Path: consts.ConfigFilePath, Value: cfg}, nil
}

func (s *Service) SaveSettingsForm(ctx context.Context, value config.Config) (*types.SettingsFormResponse, error) {
	_ = ctx
	value.MarkAdminLocalOnlyExplicit()
	value.ApplyDefaults()
	if errs := config.Validate(&value); len(errs) > 0 {
		return nil, errs[0]
	}
	body, err := yaml.Marshal(&value)
	if err != nil {
		return nil, err
	}
	if err := s.fs.MkdirAll(filepath.Dir(consts.ConfigFilePath), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(consts.ConfigFilePath, body, 0o644); err != nil {
		return nil, err
	}
	s.cfg = &value
	s.invalidateGraphCache()
	return &types.SettingsFormResponse{Path: consts.ConfigFilePath, Value: value}, nil
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

func (s *Service) DeleteMedia(ctx context.Context, reference string) error {
	if err := requireCapability(ctx, "media.lifecycle"); err != nil {
		return err
	}
	_, fullPath, err := s.resolveMediaItem(reference)
	if err != nil {
		return err
	}
	now := time.Now()
	if _, err := s.trashFile(fullPath, now); err != nil {
		return err
	}
	if err := s.trashMediaMetadataForPrimary(fullPath, now); err != nil && !os.IsNotExist(err) {
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

func toValidationDiagnostics(in []theme.ValidationDiagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toAdminThemeDiagnostics(in []adminui.Diagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toPluginDiagnostics(in []plugins.ValidationDiagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toPluginDependencies(in []plugins.Dependency) []types.PluginDependency {
	out := make([]types.PluginDependency, 0, len(in))
	for _, dep := range in {
		out = append(out, types.PluginDependency{
			Name:     dep.Name,
			Version:  dep.Version,
			Optional: dep.Optional,
		})
	}
	return out
}
