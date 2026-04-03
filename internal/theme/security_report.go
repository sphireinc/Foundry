package theme

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SecurityAssetFinding struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status,omitempty"`
}

type SecurityReport struct {
	Declared         ThemeSecurity          `json:"declared"`
	DeclaredSummary  []string               `json:"declared_summary,omitempty"`
	DetectedAssets   []SecurityAssetFinding `json:"detected_assets,omitempty"`
	DetectedRequests []SecurityAssetFinding `json:"detected_requests,omitempty"`
	Mismatches       []ValidationDiagnostic `json:"mismatches,omitempty"`
	GeneratedCSP     string                 `json:"generated_csp,omitempty"`
	CSPSummary       []string               `json:"csp_summary,omitempty"`
}

func AnalyzeInstalledSecurity(themesDir, name string) (*SecurityReport, error) {
	manifest, err := LoadManifest(themesDir, name)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(themesDir, name)
	report := &SecurityReport{
		Declared:        manifest.Security,
		DeclaredSummary: manifest.Security.Summary(),
		GeneratedCSP:    ContentSecurityPolicy(manifest),
		CSPSummary:      summarizeCSP(manifest.Security),
	}
	detectedAssets, detectedRequests, walkErr := detectRemoteThemeReferences(root, manifest.Security)
	if walkErr != nil {
		return nil, walkErr
	}
	report.DetectedAssets = detectedAssets
	report.DetectedRequests = detectedRequests
	if validation, err := ValidateInstalledDetailed(themesDir, name); err == nil {
		for _, diag := range validation.Diagnostics {
			if strings.Contains(diag.Message, "security.") || strings.Contains(diag.Message, "remote asset") || strings.Contains(diag.Message, "frontend request") {
				report.Mismatches = append(report.Mismatches, diag)
			}
		}
	}
	return report, nil
}

func detectRemoteThemeReferences(root string, sec ThemeSecurity) ([]SecurityAssetFinding, []SecurityAssetFinding, error) {
	normalizeThemeSecurity(&sec)
	assets := []SecurityAssetFinding{}
	requests := []SecurityAssetFinding{}
	seen := map[string]struct{}{}
	assetAllowlist := themeExternalAssetAllowlist(sec)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".css", ".js":
		default:
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, raw := range remoteURLPattern.FindAllString(string(body), -1) {
			key := path + "|" + raw
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			kind := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
			if strings.EqualFold(filepath.Ext(path), ".js") {
				requests = append(requests, SecurityAssetFinding{
					Kind:   kind,
					URL:    raw,
					Path:   filepath.ToSlash(path),
					Status: allowState(sec.FrontendRequests.Allowed && URLAllowedByPatterns(raw, sec.FrontendRequests.Origins)),
				})
				continue
			}
			assets = append(assets, SecurityAssetFinding{
				Kind:   kind,
				URL:    raw,
				Path:   filepath.ToSlash(path),
				Status: allowState(sec.ExternalAssets.Allowed && URLAllowedByPatterns(raw, assetAllowlist)),
			})
		}
		return nil
	})
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Path != assets[j].Path {
			return assets[i].Path < assets[j].Path
		}
		return assets[i].URL < assets[j].URL
	})
	sort.Slice(requests, func(i, j int) bool {
		if requests[i].Path != requests[j].Path {
			return requests[i].Path < requests[j].Path
		}
		return requests[i].URL < requests[j].URL
	})
	return assets, requests, err
}

func summarizeCSP(sec ThemeSecurity) []string {
	normalizeThemeSecurity(&sec)
	out := []string{
		"default-src self",
		fmt.Sprintf("script sources: %d", len(sec.ExternalAssets.Scripts)+1),
		fmt.Sprintf("style sources: %d", len(sec.ExternalAssets.Styles)+1),
	}
	if sec.FrontendRequests.Allowed && len(sec.FrontendRequests.Origins) > 0 {
		out = append(out, "connect-src includes declared remote origins")
	} else {
		out = append(out, "connect-src restricted to self")
	}
	return out
}

func allowState(ok bool) string {
	if ok {
		return "declared"
	}
	return "undeclared"
}
