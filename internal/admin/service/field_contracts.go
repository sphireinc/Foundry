package service

import (
	"path/filepath"
	"strings"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func (s *Service) activeThemeManifest() *theme.Manifest {
	if s == nil || s.cfg == nil {
		return nil
	}
	manifest, err := theme.LoadManifest(s.cfg.ThemesDir, s.cfg.Theme)
	if err != nil {
		return nil
	}
	return manifest
}

func documentFieldDefinitionsForManifest(manifest *theme.Manifest, sourcePath string, cfg *config.Config, layout, slug string) []config.FieldDefinition {
	return theme.ApplicableDocumentFieldDefinitions(manifest, documentKindFromSourcePath(sourcePath, cfg), layout, slug)
}

func documentContractsForManifest(manifest *theme.Manifest, sourcePath string, cfg *config.Config, layout, slug string) []theme.FieldContract {
	return theme.ApplicableDocumentFieldContracts(manifest, documentKindFromSourcePath(sourcePath, cfg), layout, slug)
}

func sharedFieldContractsForManifest(manifest *theme.Manifest) []admintypes.SharedFieldContract {
	contracts := theme.SharedFieldContracts(manifest)
	if len(contracts) == 0 {
		return nil
	}
	out := make([]admintypes.SharedFieldContract, 0, len(contracts))
	for _, contract := range contracts {
		out = append(out, admintypes.SharedFieldContract{
			Key:         strings.TrimSpace(contract.Target.Key),
			Title:       strings.TrimSpace(contract.Title),
			Description: strings.TrimSpace(contract.Description),
			Fields:      toFieldSchema(contract.Fields),
		})
	}
	return out
}

func documentContractMetadata(contracts []theme.FieldContract) ([]string, []string) {
	if len(contracts) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(contracts))
	titles := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		key := strings.TrimSpace(contract.Key)
		if key != "" {
			keys = append(keys, key)
		}
		title := strings.TrimSpace(contract.Title)
		if title == "" {
			title = strings.TrimSpace(contract.Key)
		}
		if title != "" {
			titles = append(titles, title)
		}
	}
	if len(keys) == 0 {
		keys = nil
	}
	if len(titles) == 0 {
		titles = nil
	}
	return keys, titles
}

func displayCustomFieldsPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "/content/"); idx >= 0 {
		return path[idx+1:]
	}
	if strings.HasPrefix(path, "content/") {
		return path
	}
	return path
}
