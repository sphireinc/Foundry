package theme

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type SecurityAssetFinding struct {
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Field       string `json:"field,omitempty"`
	Status      string `json:"status,omitempty"`
	Remediation string `json:"remediation,omitempty"`
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
			if strings.HasPrefix(diag.Code, "theme.security.") {
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
	references, err := scanThemeSecurityReferences(root)
	if err != nil {
		return nil, nil, err
	}
	for _, reference := range references {
		finding := SecurityAssetFinding{
			Kind:        reference.Kind,
			URL:         reference.URL,
			Path:        reference.Path,
			Line:        reference.Line,
			Field:       securityReferenceField(reference.Kind),
			Status:      allowState(securityReferenceAllowed(reference, sec)),
			Remediation: securityReferenceHint(reference),
		}
		if reference.Kind == "request" {
			requests = append(requests, finding)
		} else {
			assets = append(assets, finding)
		}
	}
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Path != assets[j].Path {
			return assets[i].Path < assets[j].Path
		}
		if assets[i].Line != assets[j].Line {
			return assets[i].Line < assets[j].Line
		}
		return assets[i].URL < assets[j].URL
	})
	sort.Slice(requests, func(i, j int) bool {
		if requests[i].Path != requests[j].Path {
			return requests[i].Path < requests[j].Path
		}
		if requests[i].Line != requests[j].Line {
			return requests[i].Line < requests[j].Line
		}
		return requests[i].URL < requests[j].URL
	})
	return assets, requests, nil
}

func summarizeCSP(sec ThemeSecurity) []string {
	normalizeThemeSecurity(&sec)
	out := []string{
		"default-src self",
		fmt.Sprintf("script sources: %d", len(allowedExternalSources(sec.ExternalAssets.Allowed, sec.ExternalAssets.Scripts))+1),
		fmt.Sprintf("style sources: %d", len(allowedExternalSources(sec.ExternalAssets.Allowed, sec.ExternalAssets.Styles))+1),
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
