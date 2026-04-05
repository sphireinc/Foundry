package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/customfields"
	"github.com/sphireinc/foundry/internal/plugins"
	"gopkg.in/yaml.v3"
)

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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
