package service

import (
	"context"
	"fmt"
	"strings"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/content"
)

func currentIdentity(ctx context.Context) (*adminauth.Identity, bool) {
	return adminauth.IdentityFromContext(ctx)
}

func requireCapability(ctx context.Context, capability string) error {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return nil
	}
	if !adminauthCapabilityAllowed(identity, capability) {
		return fmt.Errorf("insufficient capability: %s", capability)
	}
	return nil
}

func adminauthCapabilityAllowed(identity *adminauth.Identity, capability string) bool {
	if identity == nil {
		return false
	}
	for _, candidate := range identity.Capabilities {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		capability = strings.TrimSpace(strings.ToLower(capability))
		if candidate == "*" || candidate == capability {
			return true
		}
		if strings.HasSuffix(capability, ".own") && candidate == strings.TrimSuffix(capability, ".own") {
			return true
		}
		if candidate == capability+".own" {
			return true
		}
	}
	return false
}

func documentOwnerFromParams(params map[string]any) string {
	if params == nil {
		return ""
	}
	if value, ok := params["owner"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func documentOwnerFromFrontMatter(fm *content.FrontMatter) string {
	if fm == nil {
		return ""
	}
	if value := strings.TrimSpace(fm.Author); value != "" {
		return value
	}
	return documentOwnerFromParams(fm.Params)
}

func documentOwner(doc *content.Document) string {
	if doc == nil {
		return ""
	}
	if value := strings.TrimSpace(doc.Author); value != "" {
		return value
	}
	return documentOwnerFromParams(doc.Params)
}

func canAccessDocument(identity *adminauth.Identity, doc *content.Document) bool {
	if identity == nil || doc == nil {
		return false
	}
	if adminauthCapabilityAllowed(identity, "documents.read") {
		return true
	}
	if adminauthCapabilityAllowed(identity, "documents.read.own") {
		return strings.EqualFold(documentOwner(doc), identity.Username)
	}
	return false
}

func canMutateDocument(identity *adminauth.Identity, owner string) bool {
	if identity == nil {
		return false
	}
	if adminauthCapabilityAllowed(identity, "documents.write") || adminauthCapabilityAllowed(identity, "documents.review") || adminauthCapabilityAllowed(identity, "documents.lifecycle") {
		return true
	}
	if adminauthCapabilityAllowed(identity, "documents.write.own") || adminauthCapabilityAllowed(identity, "documents.lifecycle.own") {
		return owner != "" && strings.EqualFold(owner, identity.Username)
	}
	return false
}
