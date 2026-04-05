package httpadmin

import (
	"net/http"
	"strconv"
	"strings"

	adminaudit "github.com/sphireinc/foundry/internal/admin/audit"
	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/plugins"
)

func (r *Router) logAudit(reqActor, action, outcome, target string, reqMetadata map[string]string) {
	if r == nil || r.cfg == nil {
		return
	}
	entry := admintypes.AuditEntry{
		Action:   strings.TrimSpace(action),
		Outcome:  strings.TrimSpace(outcome),
		Target:   strings.TrimSpace(target),
		Metadata: reqMetadata,
	}
	if reqActor != "" {
		entry.Actor = reqActor
	}
	_ = adminaudit.Log(r.cfg, entry)
}

func (r *Router) handleAudit(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := adminaudit.List(r.cfg, 200)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) logAuditRequest(req *http.Request, action, outcome, target string, metadata map[string]string) {
	if r == nil || r.cfg == nil {
		return
	}
	entry := admintypes.AuditEntry{
		Action:     strings.TrimSpace(action),
		Outcome:    strings.TrimSpace(outcome),
		Target:     strings.TrimSpace(target),
		RemoteAddr: strings.TrimSpace(req.RemoteAddr),
		Metadata:   metadata,
	}
	if identity, ok := adminauth.IdentityFromContext(req.Context()); ok {
		entry.Actor = strings.TrimSpace(firstNonEmpty(identity.Name, identity.Username))
		entry.ActorRole = strings.TrimSpace(identity.Role)
	}
	_ = adminaudit.Log(r.cfg, entry)
}

func (r *Router) logPluginSecurityAudit(req *http.Request, action, pluginName string) {
	if r == nil || r.cfg == nil {
		return
	}
	meta, err := plugins.LoadMetadata(r.cfg.PluginsDir, pluginName)
	if err != nil {
		return
	}
	report := plugins.AnalyzeInstalled(meta)
	metadata := map[string]string{
		"risk_tier":         report.RiskTier,
		"requires_approval": auditBoolString(report.RequiresApproval),
		"mismatch_count":    stringInt(len(report.Mismatches)),
		"runtime_mode":      report.Runtime.Mode,
		"runtime_host":      report.Effective.RuntimeHost,
		"runtime_supported": auditBoolString(report.Effective.RuntimeSupported),
	}
	logMeta := metadata
	entry := admintypes.AuditEntry{
		Action:     strings.TrimSpace(action),
		Outcome:    "success",
		Target:     strings.TrimSpace(pluginName),
		RemoteAddr: strings.TrimSpace(req.RemoteAddr),
		Metadata:   logMeta,
	}
	if identity, ok := adminauth.IdentityFromContext(req.Context()); ok {
		entry.Actor = strings.TrimSpace(firstNonEmpty(identity.Name, identity.Username))
		entry.ActorRole = strings.TrimSpace(identity.Role)
	}
	_ = adminaudit.Log(r.cfg, entry)
	if len(report.Mismatches) > 0 {
		entry.Action = "plugin.security.mismatch"
		_ = adminaudit.Log(r.cfg, entry)
	}
}

func auditBoolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func stringInt(v int) string {
	return strconv.Itoa(v)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
