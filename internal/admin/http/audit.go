package httpadmin

import (
	"net/http"
	"strings"

	adminaudit "github.com/sphireinc/foundry/internal/admin/audit"
	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

func (r *Router) logAudit(reqActor, action, outcome, target string, reqMetadata map[string]string) {
	if r == nil || r.cfg == nil {
		return
	}
	entry := admintypes.AuditEntry{
		Action:  strings.TrimSpace(action),
		Outcome: strings.TrimSpace(outcome),
		Target:  strings.TrimSpace(target),
		Metadata: reqMetadata,
	}
	if reqActor != "" {
		entry.Actor = reqActor
	}
	_ = adminaudit.Log(r.cfg, entry)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
