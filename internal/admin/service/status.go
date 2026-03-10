package service

import (
	"context"
	"sort"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/theme"
)

func (s *Service) GetSystemStatus(ctx context.Context) (*types.SystemStatus, error) {
	status := &types.SystemStatus{
		Name:           s.cfg.Name,
		Title:          s.cfg.Title,
		BaseURL:        s.cfg.BaseURL,
		DefaultLang:    s.cfg.DefaultLang,
		PublicDir:      s.cfg.PublicDir,
		ContentDir:     s.cfg.ContentDir,
		DataDir:        s.cfg.DataDir,
		ThemesDir:      s.cfg.ThemesDir,
		PluginsDir:     s.cfg.PluginsDir,
		AdminEnabled:   s.cfg.Admin.Enabled,
		AdminLocalOnly: s.cfg.Admin.LocalOnly,
		Content: types.ContentStatus{
			ByType: make(map[string]int),
			ByLang: make(map[string]int),
		},
		Plugins:    make([]types.PluginStatus, 0),
		Taxonomies: make([]types.TaxonomyStatus, 0),
		Checks:     make([]types.HealthCheck, 0),
	}

	for _, provider := range s.providers() {
		if err := provider.Provide(ctx, s, status); err != nil {
			status.Checks = append(status.Checks, types.HealthCheck{
				Name:    provider.Name(),
				Status:  "fail",
				Message: err.Error(),
			})
		} else {
			status.Checks = append(status.Checks, types.HealthCheck{
				Name:   provider.Name(),
				Status: "ok",
			})
		}
	}

	sort.Slice(status.Plugins, func(i, j int) bool {
		return status.Plugins[i].Name < status.Plugins[j].Name
	})
	sort.Slice(status.Taxonomies, func(i, j int) bool {
		return status.Taxonomies[i].Name < status.Taxonomies[j].Name
	})
	sort.Slice(status.Checks, func(i, j int) bool {
		return status.Checks[i].Name < status.Checks[j].Name
	})

	return status, nil
}

type configStatusProvider struct{}

func (configStatusProvider) Name() string { return "config" }

func (configStatusProvider) Provide(_ context.Context, _ *Service, _ *types.SystemStatus) error {
	return nil
}

type contentStatusProvider struct{}

func (contentStatusProvider) Name() string { return "content" }

func (contentStatusProvider) Provide(ctx context.Context, s *Service, status *types.SystemStatus) error {
	graph, err := s.load(ctx, true)
	if err != nil {
		return err
	}

	status.Content.DocumentCount = len(graph.Documents)
	status.Content.RouteCount = len(graph.ByURL)

	for _, docs := range graph.ByType {
		for _, doc := range docs {
			status.Content.ByType[doc.Type]++
			status.Content.ByLang[doc.Lang]++
			if doc.Draft {
				status.Content.DraftCount++
			}
		}
		break
	}

	for typ, docs := range graph.ByType {
		status.Content.ByType[typ] = len(docs)
	}
	for lang, docs := range graph.ByLang {
		status.Content.ByLang[lang] = len(docs)
	}

	return nil
}

type themeStatusProvider struct{}

func (themeStatusProvider) Name() string { return "theme" }

func (themeStatusProvider) Provide(_ context.Context, s *Service, status *types.SystemStatus) error {
	status.Theme.Current = s.cfg.Theme
	status.Theme.Valid = false

	if err := theme.ValidateInstalled(s.cfg.ThemesDir, s.cfg.Theme); err != nil {
		return err
	}

	manifest, err := theme.LoadManifest(s.cfg.ThemesDir, s.cfg.Theme)
	if err != nil {
		return err
	}

	status.Theme.Valid = true
	status.Theme.Title = manifest.Title
	status.Theme.Version = manifest.Version
	status.Theme.Description = manifest.Description
	return nil
}

type pluginStatusProvider struct{}

func (pluginStatusProvider) Name() string { return "plugins" }

func (pluginStatusProvider) Provide(_ context.Context, s *Service, status *types.SystemStatus) error {
	installed, err := plugins.ListInstalled(s.cfg.PluginsDir)
	if err != nil {
		return err
	}

	enabledSet := make(map[string]struct{}, len(s.cfg.Plugins.Enabled))
	for _, name := range s.cfg.Plugins.Enabled {
		enabledSet[name] = struct{}{}
	}

	statuses := plugins.EnabledPluginStatus(s.cfg.PluginsDir, s.cfg.Plugins.Enabled)
	added := make(map[string]struct{})

	for _, meta := range installed {
		_, enabled := enabledSet[meta.Name]
		pstatus := statuses[meta.Name]
		if pstatus == "" {
			if enabled {
				pstatus = "enabled"
			} else {
				pstatus = "installed"
			}
		}

		status.Plugins = append(status.Plugins, types.PluginStatus{
			Name:    meta.Name,
			Title:   meta.Title,
			Version: meta.Version,
			Enabled: enabled,
			Status:  pstatus,
		})
		added[meta.Name] = struct{}{}
	}

	for _, name := range s.cfg.Plugins.Enabled {
		if _, ok := added[name]; ok {
			continue
		}
		status.Plugins = append(status.Plugins, types.PluginStatus{
			Name:    name,
			Title:   "-",
			Version: "-",
			Enabled: true,
			Status:  statuses[name],
		})
	}

	return nil
}

type taxonomyStatusProvider struct{}

func (taxonomyStatusProvider) Name() string { return "taxonomies" }

func (taxonomyStatusProvider) Provide(ctx context.Context, s *Service, status *types.SystemStatus) error {
	graph, err := s.load(ctx, true)
	if err != nil {
		return err
	}

	for name, terms := range graph.Taxonomies.Values {
		status.Taxonomies = append(status.Taxonomies, types.TaxonomyStatus{
			Name:      name,
			TermCount: len(terms),
		})
	}

	return nil
}
