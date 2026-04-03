package plugins

import (
	"fmt"
	"strings"
)

const SupportedFoundryAPI = "v1"

func validateMetadataCompatibility(meta Metadata) error {
	if strings.TrimSpace(meta.FoundryAPI) == "" {
		return fmt.Errorf("missing required field %q", "foundry_api")
	}
	if meta.FoundryAPI != SupportedFoundryAPI {
		return fmt.Errorf("unsupported foundry_api %q (supported: %s)", meta.FoundryAPI, SupportedFoundryAPI)
	}

	if strings.TrimSpace(meta.MinFoundryVersion) == "" {
		return fmt.Errorf("missing required field %q", "min_foundry_version")
	}

	for _, page := range meta.AdminExtensions.Pages {
		if strings.TrimSpace(page.NavGroup) == "" {
			continue
		}
		switch normalizeAdminNavGroup(page.NavGroup) {
		case "dashboard", "content", "manage", "admin":
		default:
			return fmt.Errorf("unsupported admin.pages.nav_group %q for page %q (supported: dashboard, content, manage, admin)", page.NavGroup, page.Key)
		}
	}

	if err := validatePermissions(meta.Permissions); err != nil {
		return err
	}
	if err := validateRuntime(meta.Runtime); err != nil {
		return err
	}

	return nil
}

func validatePermissions(p PermissionSet) error {
	for _, method := range p.Network.Outbound.Methods {
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		default:
			return fmt.Errorf("unsupported permissions.network.outbound.methods value %q", method)
		}
	}
	if !p.Process.Exec.Allowed && len(p.Process.Exec.Commands) > 0 {
		return fmt.Errorf("permissions.process.exec.allowed must be true when commands are declared")
	}
	if !p.Environment.Read.Allowed && len(p.Environment.Read.Variables) > 0 {
		return fmt.Errorf("permissions.environment.read.allowed must be true when variables are declared")
	}
	if (p.Network.Outbound.HTTP || p.Network.Outbound.HTTPS || p.Network.Outbound.WebSocket || p.Network.Outbound.GRPC) && len(p.Network.Outbound.Methods) == 0 {
		return fmt.Errorf("permissions.network.outbound.methods must declare at least one HTTP method when outbound network access is enabled")
	}
	if !p.Capabilities.RequiresAdminApproval && (p.Capabilities.Dangerous ||
		p.Process.Exec.Allowed ||
		p.Process.Shell.Allowed ||
		p.Process.SpawnBackground.Allowed ||
		p.Secrets.Access.AdminTokens ||
		p.Secrets.Access.SessionStore ||
		p.Secrets.Access.PasswordHashes ||
		p.Secrets.Access.TOTPSecrets ||
		p.Secrets.Access.EnvSecrets ||
		p.Secrets.Access.DeployKeys ||
		p.Secrets.Access.UpdateCredentials) {
		return fmt.Errorf("dangerous or secret-accessing plugins must set permissions.capabilities.requires_admin_approval=true")
	}
	return nil
}

func validateRuntime(r RuntimeConfig) error {
	switch strings.TrimSpace(r.Mode) {
	case "", "in_process", "rpc":
	default:
		return fmt.Errorf("unsupported runtime.mode %q (supported: in_process, rpc)", r.Mode)
	}
	switch strings.TrimSpace(r.Sandbox.Profile) {
	case "", "default", "strict":
	default:
		return fmt.Errorf("unsupported runtime.sandbox.profile %q (supported: default, strict)", r.Sandbox.Profile)
	}
	if strings.TrimSpace(r.Mode) == "rpc" && len(r.Command) == 0 && strings.TrimSpace(r.Socket) == "" {
		return fmt.Errorf("runtime.mode rpc must declare runtime.command or runtime.socket")
	}
	return nil
}
