package managed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

func TestBuildHealthReportHealthy(t *testing.T) {
	cfg := testHealthConfig(t)
	cfg.Foundry.Managed.Enabled = true
	cfg.Foundry.Managed.InstanceID = "instance-123"
	cfg.Admin.Enabled = true
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	report := BuildHealthReport(cfg, HealthVersion{Version: "v1.3.9", Commit: "abc1234"}, now)
	if report.Status != HealthStatusHealthy {
		t.Fatalf("expected healthy status, got %#v", report)
	}
	if !report.Managed {
		t.Fatal("expected managed=true")
	}
	if report.InstanceID != "instance-123" {
		t.Fatalf("expected configured instance ID, got %q", report.InstanceID)
	}
	if !report.AdminReady {
		t.Fatal("expected admin ready")
	}
	if report.Version != "v1.3.9" || report.Commit != "abc1234" {
		t.Fatalf("unexpected version metadata: %#v", report)
	}
	for _, check := range report.Checks {
		if check.Status != HealthCheckPass {
			t.Fatalf("expected all checks to pass, got %#v", report.Checks)
		}
		if strings.Contains(check.Message, string(filepath.Separator)) {
			t.Fatalf("health message leaks filesystem detail: %#v", check)
		}
	}
}

func TestBuildHealthReportDegraded(t *testing.T) {
	cfg := testHealthConfig(t)
	cfg.Admin.Enabled = false
	cfg.PublicDir = filepath.Join(t.TempDir(), "missing-public")

	report := BuildHealthReport(cfg, HealthVersion{}, time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC))
	if report.Status != HealthStatusDegraded {
		t.Fatalf("expected degraded status, got %#v", report)
	}
	if report.Managed {
		t.Fatal("expected managed=false")
	}
	if report.AdminReady {
		t.Fatal("expected admin not ready")
	}
	if report.Version != "unknown" || report.Commit != "unknown" {
		t.Fatalf("expected unknown version defaults, got %#v", report)
	}
	foundAdminFailure := false
	foundStorageFailure := false
	for _, check := range report.Checks {
		if check.Name == "admin" && check.Status == HealthCheckFail {
			foundAdminFailure = true
		}
		if check.Name == "storage.public" && check.Status == HealthCheckFail {
			foundStorageFailure = true
			if strings.Contains(check.Message, cfg.PublicDir) {
				t.Fatalf("storage failure leaked directory path: %#v", check)
			}
		}
	}
	if !foundAdminFailure || !foundStorageFailure {
		t.Fatalf("expected admin and public storage failures, got %#v", report.Checks)
	}
}

func TestBuildHealthReportReadsBootstrapInstanceID(t *testing.T) {
	cfg := testHealthConfig(t)
	cfg.Admin.Enabled = true
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	_, err := ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: cfg.DataDir,
		Payload: validBootstrapPayload(now),
		Now:     now,
		Apply: func(BootstrapPayload) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("apply bootstrap: %v", err)
	}

	report := BuildHealthReport(cfg, HealthVersion{}, now)
	if report.InstanceID != "instance-456" {
		t.Fatalf("expected bootstrap instance ID, got %q", report.InstanceID)
	}
}

func TestCheckStorageLayoutReportsRequiredWritablePaths(t *testing.T) {
	cfg := testHealthConfig(t)
	checks := CheckStorageLayout(cfg)
	if len(checks) != len(RuntimeStorageLayout()) {
		t.Fatalf("expected one check per layout entry, got %#v", checks)
	}
	for _, check := range checks {
		if check.Status != HealthCheckPass {
			t.Fatalf("expected storage check to pass: %#v", check)
		}
	}

	cfg.PublicDir = filepath.Join(t.TempDir(), "missing")
	checks = CheckStorageLayout(cfg)
	foundPublicFailure := false
	for _, check := range checks {
		if check.Name == "storage.public" {
			foundPublicFailure = true
			if check.Status != HealthCheckFail {
				t.Fatalf("expected public storage failure, got %#v", check)
			}
			if strings.Contains(check.Message, cfg.PublicDir) {
				t.Fatalf("storage layout check leaked path: %#v", check)
			}
		}
	}
	if !foundPublicFailure {
		t.Fatalf("expected public storage check, got %#v", checks)
	}
}

func testHealthConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		PluginsDir:  filepath.Join(root, "plugins"),
		Admin: config.AdminConfig{
			Enabled: true,
			Path:    "/__admin",
		},
	}
	cfg.ApplyDefaults()
	for _, dir := range []string{cfg.ContentDir, cfg.PublicDir, cfg.ThemesDir, cfg.DataDir, cfg.PluginsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return cfg
}
