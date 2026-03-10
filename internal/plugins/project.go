package plugins

import "strings"

type Project struct {
	ConfigPath string
	PluginsDir string
	OutputPath string
	ModulePath string
}

type MissingDependency struct {
	Repo      string
	Installed bool
	Name      string
}

type DependencyStatus struct {
	Repo   string
	Status string
	Name   string
}

func NewProject(configPath, pluginsDir, outputPath, modulePath string) Project {
	return Project{
		ConfigPath: strings.TrimSpace(configPath),
		PluginsDir: strings.TrimSpace(pluginsDir),
		OutputPath: strings.TrimSpace(outputPath),
		ModulePath: strings.TrimSpace(modulePath),
	}
}

func (p Project) Sync() error {
	return SyncFromConfig(SyncOptions{
		ConfigPath: p.ConfigPath,
		PluginsDir: p.PluginsDir,
		OutputPath: p.OutputPath,
		ModulePath: p.ModulePath,
	})
}

func (p Project) Install(url, name string) (Metadata, error) {
	return Install(InstallOptions{
		PluginsDir: p.PluginsDir,
		URL:        strings.TrimSpace(url),
		Name:       strings.TrimSpace(name),
	})
}

func (p Project) Uninstall(name string) error {
	return Uninstall(p.PluginsDir, name)
}

func (p Project) Enable(name string) error {
	return EnableInConfig(p.ConfigPath, name)
}

func (p Project) Disable(name string) error {
	return DisableInConfig(p.ConfigPath, name)
}

func (p Project) Update(name string) (Metadata, error) {
	return UpdateInstalled(p.PluginsDir, name)
}

func (p Project) Validate(name string) error {
	return ValidateInstalledPlugin(p.PluginsDir, name)
}

func (p Project) ValidateEnabled(enabled []string) []ValidationIssue {
	return ValidateEnabledPlugins(p.PluginsDir, enabled)
}

func (p Project) EnabledStatuses(enabled []string) map[string]string {
	return EnabledPluginStatus(p.PluginsDir, enabled)
}

func (p Project) ListInstalled() ([]Metadata, error) {
	return ListInstalled(p.PluginsDir)
}

func (p Project) Metadata(name string) (Metadata, error) {
	return LoadMetadata(p.PluginsDir, name)
}

func (p Project) MissingDependencies(installed Metadata, enabled []string) ([]MissingDependency, error) {
	if len(installed.Requires) == 0 {
		return nil, nil
	}

	enabledMetadata, err := LoadAllMetadata(p.PluginsDir, enabled)
	if err != nil {
		return nil, err
	}

	enabledRepos := make(map[string]string, len(enabledMetadata))
	for name, meta := range enabledMetadata {
		repo := strings.TrimSpace(meta.Repo)
		if repo == "" {
			continue
		}
		enabledRepos[repo] = name
	}

	installedOnDisk, err := p.scanInstalledPluginRepos()
	if err != nil {
		return nil, err
	}

	missing := make([]MissingDependency, 0)
	seen := make(map[string]struct{})

	for _, dep := range installed.Requires {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := enabledRepos[dep]; ok {
			continue
		}
		if _, dup := seen[dep]; dup {
			continue
		}
		seen[dep] = struct{}{}

		md := MissingDependency{Repo: dep}
		if name, ok := installedOnDisk[dep]; ok {
			md.Installed = true
			md.Name = name
		}

		missing = append(missing, md)
	}

	return missing, nil
}

func (p Project) DependencyStatuses(name string, enabled []string) ([]DependencyStatus, error) {
	meta, err := LoadMetadata(p.PluginsDir, name)
	if err != nil {
		return nil, err
	}

	if len(meta.Requires) == 0 {
		return nil, nil
	}

	enabledMetadata, err := LoadAllMetadata(p.PluginsDir, enabled)
	if err != nil {
		return nil, err
	}

	enabledRepos := make(map[string]string, len(enabledMetadata))
	for pluginName, m := range enabledMetadata {
		if repo := strings.TrimSpace(m.Repo); repo != "" {
			enabledRepos[repo] = pluginName
		}
	}

	installed, err := p.ListInstalled()
	if err != nil {
		return nil, err
	}

	installedRepos := make(map[string]string, len(installed))
	for _, m := range installed {
		if repo := strings.TrimSpace(m.Repo); repo != "" {
			installedRepos[repo] = m.Name
		}
	}

	out := make([]DependencyStatus, 0, len(meta.Requires))
	for _, dep := range meta.Requires {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}

		status := DependencyStatus{Repo: dep}

		switch {
		case enabledRepos[dep] != "":
			status.Status = "enabled"
			status.Name = enabledRepos[dep]
		case installedRepos[dep] != "":
			status.Status = "installed"
			status.Name = installedRepos[dep]
		default:
			status.Status = "missing"
		}

		out = append(out, status)
	}

	return out, nil
}

func (p Project) scanInstalledPluginRepos() (map[string]string, error) {
	metas, err := p.ListInstalled()
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, meta := range metas {
		repo := strings.TrimSpace(meta.Repo)
		if repo == "" {
			continue
		}
		out[repo] = meta.Name
	}

	return out, nil
}
