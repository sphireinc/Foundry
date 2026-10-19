package mcp

import (
	"time"

	adminaudit "github.com/sphireinc/foundry/internal/admin/audit"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
)

type Auditor interface {
	Log(admintypes.AuditEntry) error
}

type noopAuditor struct{}

func (noopAuditor) Log(admintypes.AuditEntry) error { return nil }

type fileAuditor struct {
	cfg *config.Config
}

func (a *fileAuditor) Log(entry admintypes.AuditEntry) error {
	if a == nil || a.cfg == nil {
		return nil
	}
	return adminaudit.Log(a.cfg, entry)
}

func NewAuditor(cfg *config.Config, enabled bool) Auditor {
	if !enabled || cfg == nil {
		return noopAuditor{}
	}
	return &fileAuditor{cfg: cfg}
}

func AuditEntry(actor Actor, capability Capability, target, outcome string, extra map[string]string) admintypes.AuditEntry {
	metadata := map[string]string{
		"capability": capability.Name,
		"scope":      string(capability.Scope),
		"transport":  "mcp",
	}
	if capability.Plugin != "" {
		metadata["plugin"] = capability.Plugin
	}
	for key, value := range extra {
		metadata[key] = value
	}

	return admintypes.AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    "mcp." + capability.AuditCategory,
		Actor:     actor.ID,
		ActorRole: actor.Role,
		Target:    target,
		Outcome:   outcome,
		Metadata:  metadata,
	}
}
