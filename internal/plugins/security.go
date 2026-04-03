package plugins

import (
	"sort"
	"strings"
)

func normalizePermissionSet(p *PermissionSet) {
	if p == nil {
		return
	}
	p.Filesystem.Read.Custom = normalizeStringList(p.Filesystem.Read.Custom)
	p.Filesystem.Write.Custom = normalizeStringList(p.Filesystem.Write.Custom)
	p.Filesystem.Delete.Custom = normalizeStringList(p.Filesystem.Delete.Custom)
	p.Network.Outbound.CustomSchemes = normalizeStringList(p.Network.Outbound.CustomSchemes)
	p.Network.Outbound.Domains = normalizeStringList(p.Network.Outbound.Domains)
	p.Network.Outbound.Methods = normalizeHTTPMethodList(p.Network.Outbound.Methods)
	if len(p.Network.Outbound.Methods) == 0 {
		p.Network.Outbound.Methods = []string{"GET"}
	}
	p.Process.Exec.Commands = normalizeStringList(p.Process.Exec.Commands)
	p.Environment.Read.Variables = normalizeStringList(p.Environment.Read.Variables)
}

func normalizeRuntimeConfig(r *RuntimeConfig) {
	if r == nil {
		return
	}
	r.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
	if r.Mode == "" {
		r.Mode = "in_process"
	}
	r.ProtocolVersion = strings.TrimSpace(r.ProtocolVersion)
	if r.ProtocolVersion == "" {
		r.ProtocolVersion = "v1alpha1"
	}
	r.Command = normalizeOrderedStringList(r.Command)
	r.Socket = strings.TrimSpace(r.Socket)
	if r.Env == nil {
		r.Env = map[string]string{}
	}
	cleanEnv := make(map[string]string, len(r.Env))
	for key, value := range r.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		cleanEnv[key] = strings.TrimSpace(value)
	}
	r.Env = cleanEnv
	r.Sandbox.Profile = strings.ToLower(strings.TrimSpace(r.Sandbox.Profile))
	if r.Sandbox.Profile == "" {
		r.Sandbox.Profile = "default"
	}
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func normalizeOrderedStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeHTTPMethodList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.ToUpper(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (p PermissionSet) RiskTier() string {
	if p.Capabilities.Dangerous ||
		p.Capabilities.RequiresAdminApproval ||
		p.Process.Exec.Allowed ||
		p.Process.Shell.Allowed ||
		p.Process.SpawnBackground.Allowed ||
		p.Secrets.Access.AdminTokens ||
		p.Secrets.Access.SessionStore ||
		p.Secrets.Access.PasswordHashes ||
		p.Secrets.Access.TOTPSecrets ||
		p.Secrets.Access.EnvSecrets ||
		p.Secrets.Access.DeployKeys ||
		p.Secrets.Access.UpdateCredentials ||
		p.Admin.Operations.Updates ||
		p.Admin.Users.Write ||
		p.Admin.Users.ResetPasswords ||
		p.Admin.Users.RevokeSessions {
		return "high"
	}
	if p.Network.Outbound.HTTP || p.Network.Outbound.HTTPS || p.Network.Outbound.WebSocket || p.Network.Outbound.GRPC ||
		p.Network.Inbound.RegisterRoutes || p.Network.Inbound.AdminRoutes || p.Network.Inbound.PublicRoutes ||
		p.Filesystem.Write.Content || p.Filesystem.Write.Data || p.Filesystem.Write.Public || p.Filesystem.Write.Cache || p.Filesystem.Write.Backups ||
		p.Filesystem.Delete.Content || p.Filesystem.Delete.Data || p.Filesystem.Delete.Public || p.Filesystem.Delete.Cache || p.Filesystem.Delete.Backups ||
		p.Content.Documents.Write || p.Content.Documents.Delete || p.Content.Documents.Workflow ||
		p.Content.Media.Write || p.Content.Media.Delete || p.Content.Media.Metadata ||
		p.Config.Write.Site || p.Config.Write.PluginConfig || p.Config.Write.ThemeManifest ||
		p.Admin.Operations.Backups || p.Admin.Operations.Rebuild || p.Admin.Operations.ClearCache {
		return "medium"
	}
	return "low"
}

func (p PermissionSet) Summary() []string {
	out := []string{}
	if p.Content.Documents.Read {
		out = append(out, "Reads documents")
	}
	if p.Content.Media.Read {
		out = append(out, "Reads media")
	}
	if p.Render.Context.Write {
		out = append(out, "Mutates render context")
	}
	if p.Render.HTMLSlots.Inject {
		out = append(out, "Injects HTML slots")
	}
	if p.Render.Assets.InjectCSS || p.Render.Assets.InjectJS || p.Render.Assets.InjectRemoteAssets {
		out = append(out, "Injects assets")
	}
	if p.Graph.Read || p.Graph.Routes.Inspect || p.Graph.Taxonomies.Inspect {
		out = append(out, "Inspects site graph")
	}
	if p.Network.Outbound.HTTP || p.Network.Outbound.HTTPS || p.Network.Outbound.WebSocket || p.Network.Outbound.GRPC {
		out = append(out, "Makes outbound network requests")
	}
	if p.Network.Inbound.RegisterRoutes || p.Network.Inbound.AdminRoutes || p.Network.Inbound.PublicRoutes {
		out = append(out, "Registers routes")
	}
	if p.Process.Exec.Allowed || p.Process.Shell.Allowed || p.Process.SpawnBackground.Allowed {
		out = append(out, "Runs local processes")
	}
	if p.Capabilities.RequiresAdminApproval {
		out = append(out, "Requires admin approval")
	}
	if len(out) == 0 {
		out = append(out, "No elevated permissions declared")
	}
	return out
}

func (r RuntimeConfig) Summary() []string {
	out := []string{"Runtime: " + RuntimeModeLabel(r.Mode)}
	if r.Mode == "rpc" {
		if len(r.Command) > 0 {
			out = append(out, "RPC command configured")
		}
		if r.Socket != "" {
			out = append(out, "RPC socket declared")
		}
		out = append(out, "Sandbox profile: "+strings.TrimSpace(r.Sandbox.Profile))
	}
	return out
}

func RuntimeModeLabel(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rpc":
		return "out-of-process RPC"
	default:
		return "in-process"
	}
}
