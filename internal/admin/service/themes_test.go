package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListThemesIncludesSecurityRemediation(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Theme = "default"
	writeServiceTheme(t, cfg, cfg.Theme)

	headPath := filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "head.html")
	headBody, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("read theme head: %v", err)
	}
	headBody = []byte(strings.Replace(string(headBody), `{{ pluginSlot "head.end" }}`, `{{ pluginSlot "head.end" }}<script src="https://cdn.example.com/theme.js"></script>`, 1))
	if err := os.WriteFile(headPath, headBody, 0o644); err != nil {
		t.Fatalf("write theme head: %v", err)
	}

	themes, err := New(cfg).ListThemes(context.Background())
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	for _, candidate := range themes {
		if candidate.Name != cfg.Theme || candidate.Kind != "frontend" {
			continue
		}
		if candidate.Valid {
			t.Fatal("expected the undeclared script to make the theme invalid")
		}
		if candidate.SecurityReport == nil || len(candidate.SecurityReport.DetectedAssets) != 1 {
			t.Fatalf("expected one detected remote asset, got %#v", candidate.SecurityReport)
		}
		finding := candidate.SecurityReport.DetectedAssets[0]
		if finding.Field != "security.external_assets.scripts" || finding.Line == 0 || !strings.Contains(finding.Remediation, "theme.yaml") {
			t.Fatalf("expected script remediation metadata, got %#v", finding)
		}
		var diagnosticFound bool
		for _, diagnostic := range candidate.Diagnostics {
			if diagnostic.Code == "theme.security.remote_asset_undeclared" {
				diagnosticFound = true
				if diagnostic.Field != "security.external_assets.scripts" || diagnostic.Hint == "" {
					t.Fatalf("expected diagnostic remediation metadata, got %#v", diagnostic)
				}
			}
		}
		if !diagnosticFound {
			t.Fatalf("expected security diagnostic, got %#v", candidate.Diagnostics)
		}
		return
	}
	t.Fatalf("frontend theme %q not found in %#v", cfg.Theme, themes)
}
