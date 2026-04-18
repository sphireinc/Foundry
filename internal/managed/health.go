package managed

import (
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

const (
	HealthStatusHealthy  = "healthy"
	HealthStatusDegraded = "degraded"
	HealthCheckPass      = "pass"
	HealthCheckFail      = "fail"
)

type HealthVersion struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

type HealthReport struct {
	Status      string        `json:"status"`
	Version     string        `json:"version"`
	Commit      string        `json:"commit"`
	Managed     bool          `json:"managed"`
	InstanceID  string        `json:"instance_id,omitempty"`
	AdminReady  bool          `json:"admin_ready"`
	GeneratedAt time.Time     `json:"generated_at"`
	Checks      []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func BuildHealthReport(cfg *config.Config, version HealthVersion, now time.Time) HealthReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	report := HealthReport{
		Status:      HealthStatusHealthy,
		Version:     strings.TrimSpace(version.Version),
		Commit:      strings.TrimSpace(version.Commit),
		Managed:     cfg != nil && cfg.ManagedRuntimeEnabled(),
		AdminReady:  adminReady(cfg),
		GeneratedAt: now.UTC(),
	}
	if report.Version == "" {
		report.Version = "unknown"
	}
	if report.Commit == "" {
		report.Commit = "unknown"
	}
	if cfg == nil {
		report.addCheck("config", HealthCheckFail, "configuration is unavailable")
		report.Status = HealthStatusDegraded
		return report
	}
	report.InstanceID = managedInstanceID(cfg)
	report.addBoolCheck("admin", report.AdminReady, "admin is ready", "admin is not ready")
	for _, check := range CheckStorageLayout(cfg) {
		report.addCheck(check.Name, check.Status, check.Message)
	}
	for _, check := range report.Checks {
		if check.Status != HealthCheckPass {
			report.Status = HealthStatusDegraded
			break
		}
	}
	return report
}

func (h *HealthReport) addBoolCheck(name string, ok bool, passMessage, failMessage string) {
	if ok {
		h.addCheck(name, HealthCheckPass, passMessage)
		return
	}
	h.addCheck(name, HealthCheckFail, failMessage)
}

func (h *HealthReport) addCheck(name, status, message string) {
	h.Checks = append(h.Checks, HealthCheck{Name: name, Status: status, Message: message})
}

func adminReady(cfg *config.Config) bool {
	return cfg != nil && cfg.Admin.Enabled && strings.TrimSpace(cfg.AdminPath()) != ""
}

func managedInstanceID(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if id := strings.TrimSpace(cfg.Foundry.Managed.InstanceID); id != "" {
		return id
	}
	if state, err := ReadBootstrapState(cfg.DataDir); err == nil {
		return strings.TrimSpace(state.InstanceID)
	}
	return ""
}
