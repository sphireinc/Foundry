package plugins

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SecurityReport struct {
	DeclaredPermissions PermissionSet            `json:"declared_permissions,omitempty"`
	Runtime             RuntimeConfig            `json:"runtime,omitempty"`
	RiskTier            string                   `json:"risk_tier"`
	RequiresApproval    bool                     `json:"requires_approval"`
	Summary             []string                 `json:"summary,omitempty"`
	Findings            []SecurityFinding        `json:"findings,omitempty"`
	Mismatches          []ValidationDiagnostic   `json:"mismatches,omitempty"`
	Effective           SecurityEnforcementState `json:"effective"`
}

type SecurityFinding struct {
	Category     string `json:"category"`
	Evidence     string `json:"evidence"`
	EvidenceType string `json:"evidence_type,omitempty"`
	Path         string `json:"path,omitempty"`
	Message      string `json:"message,omitempty"`
}

type SecurityEnforcementState struct {
	Mode               string   `json:"mode,omitempty"`
	RuntimeHost        string   `json:"runtime_host,omitempty"`
	RuntimeSupported   bool     `json:"runtime_supported"`
	Strict             bool     `json:"strict"`
	Allowed            bool     `json:"allowed"`
	ApprovalRequired   bool     `json:"approval_required"`
	DeniedReasons      []string `json:"denied_reasons,omitempty"`
	CapabilityBoundary []string `json:"capability_boundary,omitempty"`
}

func AnalyzeInstalled(meta Metadata) SecurityReport {
	report := SecurityReport{
		DeclaredPermissions: meta.Permissions,
		Runtime:             meta.Runtime,
		RiskTier:            meta.Permissions.RiskTier(),
		RequiresApproval:    meta.Permissions.Capabilities.RequiresAdminApproval,
		Summary:             append(append([]string{}, meta.Permissions.Summary()...), meta.Runtime.Summary()...),
		Findings:            []SecurityFinding{},
		Mismatches:          []ValidationDiagnostic{},
		Effective: SecurityEnforcementState{
			Mode:             strings.TrimSpace(meta.Runtime.Mode),
			RuntimeHost:      ResolveRuntimeHost(meta).Name(),
			RuntimeSupported: EnsureRuntimeSupported(meta) == nil,
			Strict:           true,
			Allowed:          true,
		},
	}

	findings := analyzePluginDirectory(meta.Directory)
	report.Findings = findings
	report.Mismatches = compareDeclaredPermissions(meta, findings)
	if len(report.Mismatches) > 0 {
		report.RequiresApproval = true
		if report.RiskTier == "low" {
			report.RiskTier = "medium"
		}
	}
	if strings.EqualFold(meta.Runtime.Mode, "rpc") {
		report.RequiresApproval = true
		if report.RiskTier == "low" {
			report.RiskTier = "medium"
		}
	}
	report.Effective.ApprovalRequired = report.RequiresApproval
	if err := EnsureRuntimeSupported(meta); err != nil {
		report.Effective.RuntimeSupported = false
		report.Effective.Allowed = false
		report.Effective.DeniedReasons = append(report.Effective.DeniedReasons, err.Error())
	}
	if len(report.Mismatches) > 0 {
		report.Effective.Allowed = false
		report.Effective.DeniedReasons = append(report.Effective.DeniedReasons, "declared permissions do not match detected capabilities")
	}
	report.Effective.CapabilityBoundary = capabilityBoundaryForRuntime(meta.Runtime)
	return report
}

func analyzePluginDirectory(root string) []SecurityFinding {
	fset := token.NewFileSet()
	findings := []SecurityFinding{}
	seen := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			addFinding(&findings, seen, "parser", filepath.ToSlash(path), "parse", parseErr.Error(), "direct")
			return nil
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			alias := filepath.Base(importPath)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			imports[alias] = importPath
			switch importPath {
			case "net/http":
				addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), "import net/http", "uses net/http", "heuristic")
			case "os/exec":
				addFinding(&findings, seen, "process.exec", filepath.ToSlash(path), "import os/exec", "uses os/exec", "heuristic")
			case "plugin":
				addFinding(&findings, seen, "capabilities.dynamic_loading", filepath.ToSlash(path), "import plugin", "uses Go dynamic plugin loading", "direct")
			case "net/url":
				addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), "import net/url", "constructs or parses remote URLs", "heuristic")
			case "syscall", "unsafe":
				addFinding(&findings, seen, "capabilities.dangerous", filepath.ToSlash(path), "import "+importPath, "uses low-level unsafe package", "direct")
			}
			if strings.Contains(importPath, "grpc") {
				addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), "import "+importPath, "uses gRPC package", "heuristic")
			}
			if strings.Contains(importPath, "websocket") {
				addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), "import "+importPath, "uses websocket package", "heuristic")
			}
			if strings.Contains(importPath, "/internal/backup") {
				addFinding(&findings, seen, "admin.operations.backups", filepath.ToSlash(path), "import "+importPath, "touches backup APIs", "direct")
			}
			if strings.Contains(importPath, "/internal/updater") {
				addFinding(&findings, seen, "admin.operations.updates", filepath.ToSlash(path), "import "+importPath, "touches update APIs", "direct")
			}
			if strings.Contains(importPath, "/internal/admin/audit") {
				addFinding(&findings, seen, "admin.audit.read", filepath.ToSlash(path), "import "+importPath, "touches admin audit APIs", "direct")
			}
			if strings.Contains(importPath, "/internal/admin/users") {
				addFinding(&findings, seen, "admin.users.read", filepath.ToSlash(path), "import "+importPath, "touches admin users APIs", "direct")
			}
			if strings.Contains(importPath, "/internal/admin/auth") {
				addFinding(&findings, seen, "admin.users.reset_passwords", filepath.ToSlash(path), "import "+importPath, "touches admin auth/session APIs", "heuristic")
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			switch fn.Name.Name {
			case "RegisterRoutes":
				addFinding(&findings, seen, "network.inbound.register_routes", filepath.ToSlash(path), "RegisterRoutes", "registers preview routes", "direct")
			case "OnContext":
				addFinding(&findings, seen, "render.context", filepath.ToSlash(path), "OnContext", "reads or mutates render context", "direct")
			case "OnAssets":
				addFinding(&findings, seen, "render.assets", filepath.ToSlash(path), "OnAssets", "injects assets into render pipeline", "direct")
			case "OnHTMLSlots":
				addFinding(&findings, seen, "render.html_slots", filepath.ToSlash(path), "OnHTMLSlots", "injects HTML into theme slots", "direct")
			case "OnAfterRender":
				addFinding(&findings, seen, "render.after_render", filepath.ToSlash(path), "OnAfterRender", "mutates final rendered HTML", "direct")
			case "OnRoutesAssigned":
				addFinding(&findings, seen, "graph.mutate", filepath.ToSlash(path), "OnRoutesAssigned", "observes or mutates assigned routes", "direct")
			case "OnGraphBuilding", "OnGraphBuilt":
				addFinding(&findings, seen, "graph.read", filepath.ToSlash(path), fn.Name.Name, "observes site graph", "direct")
			case "OnTaxonomyBuilt":
				addFinding(&findings, seen, "graph.taxonomies.inspect", filepath.ToSlash(path), fn.Name.Name, "observes taxonomy graph", "direct")
			case "OnDocumentParsed", "OnFrontmatterParsed", "OnMarkdownRendered":
				addFinding(&findings, seen, "content.documents.read", filepath.ToSlash(path), fn.Name.Name, "reads document content", "direct")
			case "OnServerStarted":
				addFinding(&findings, seen, "runtime.server.on_started", filepath.ToSlash(path), fn.Name.Name, "hooks server startup", "direct")
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.BasicLit:
					if n.Kind == token.STRING {
						value := strings.Trim(n.Value, `"`)
						if looksLikeSecretPath(value) {
							addFinding(&findings, seen, "secrets.path_access", filepath.ToSlash(path), value, "references secret-looking path or file name", "heuristic")
						}
						switch {
						case strings.Contains(value, "/api/backups"):
							addFinding(&findings, seen, "admin.operations.backups", filepath.ToSlash(path), value, "references backup admin API", "heuristic")
						case strings.Contains(value, "/api/update"):
							addFinding(&findings, seen, "admin.operations.updates", filepath.ToSlash(path), value, "references update admin API", "heuristic")
						case strings.Contains(value, "/api/operations/rebuild"):
							addFinding(&findings, seen, "admin.operations.rebuild", filepath.ToSlash(path), value, "references rebuild admin API", "heuristic")
						case strings.Contains(value, "/api/operations/cache/clear"):
							addFinding(&findings, seen, "admin.operations.clear_cache", filepath.ToSlash(path), value, "references cache-clear admin API", "heuristic")
						case strings.Contains(value, "/api/audit"):
							addFinding(&findings, seen, "admin.audit.read", filepath.ToSlash(path), value, "references audit admin API", "heuristic")
						case strings.Contains(value, "/api/users"):
							addFinding(&findings, seen, "admin.users.read", filepath.ToSlash(path), value, "references users admin API", "heuristic")
						}
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				importPath := imports[ident.Name]
				if importPath == "" {
					return true
				}
				name := importPath + "." + sel.Sel.Name
				switch {
				case name == "os.ReadFile" || name == "os.Open" || name == "io/ioutil.ReadFile" || name == "io/ioutil.ReadDir" || name == "path/filepath.Walk" || name == "path/filepath.WalkDir":
					addFinding(&findings, seen, "filesystem.read", filepath.ToSlash(path), name, "reads from filesystem", "direct")
				case name == "os.WriteFile" || name == "io/ioutil.WriteFile" || name == "os.OpenFile" || name == "os.Rename":
					addFinding(&findings, seen, "filesystem.write", filepath.ToSlash(path), name, "writes to filesystem", "direct")
				case name == "os.Remove" || name == "os.RemoveAll":
					addFinding(&findings, seen, "filesystem.delete", filepath.ToSlash(path), name, "deletes filesystem paths", "direct")
				case name == "os.Getenv" || name == "os.Environ":
					addFinding(&findings, seen, "environment.read", filepath.ToSlash(path), name, "reads environment variables", "direct")
				case name == "net/http.Get" || name == "net/http.Post" || name == "net/http.NewRequest" || name == "net/http.NewRequestWithContext" || name == "net.Dial" || name == "net.DialTimeout":
					addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), name, "makes outbound network calls", "direct")
				case name == "os/exec.Command" || name == "os/exec.CommandContext":
					addFinding(&findings, seen, "process.exec", filepath.ToSlash(path), name, "executes local commands", "direct")
					if len(call.Args) > 0 {
						if lit, ok := call.Args[0].(*ast.BasicLit); ok {
							value := strings.Trim(lit.Value, `"`)
							if value == "sh" || value == "bash" || value == "zsh" || value == "cmd" || value == "powershell" {
								addFinding(&findings, seen, "process.shell", filepath.ToSlash(path), name+"("+value+")", "executes shell process", "direct")
							}
						}
					}
				case strings.HasSuffix(name, ".Client.Do"):
					addFinding(&findings, seen, "network.outbound", filepath.ToSlash(path), name, "makes outbound network calls through http.Client", "direct")
				case strings.HasSuffix(name, "/internal/admin/users.Save"):
					addFinding(&findings, seen, "admin.users.write", filepath.ToSlash(path), name, "mutates admin users", "direct")
				case strings.HasSuffix(name, "/internal/admin/users.Load") || strings.HasSuffix(name, "/internal/admin/users.Find"):
					addFinding(&findings, seen, "admin.users.read", filepath.ToSlash(path), name, "reads admin users", "direct")
				case strings.Contains(name, "/internal/admin/auth.") && (strings.HasSuffix(name, ".StartPasswordReset") || strings.HasSuffix(name, ".CompletePasswordReset")):
					addFinding(&findings, seen, "admin.users.reset_passwords", filepath.ToSlash(path), name, "resets admin passwords", "direct")
				case strings.Contains(name, "/internal/admin/auth.") && (strings.HasSuffix(name, ".RevokeSessions") || strings.HasSuffix(name, ".RevokeAllSessions")):
					addFinding(&findings, seen, "admin.users.revoke_sessions", filepath.ToSlash(path), name, "revokes admin sessions", "direct")
				case strings.HasSuffix(name, "/internal/backup.CreateManagedSnapshot") || strings.HasSuffix(name, "/internal/backup.CreateZipSnapshot") || strings.HasSuffix(name, "/internal/backup.RestoreZipSnapshot") || strings.HasSuffix(name, "/internal/backup.CreateGitSnapshot") || strings.HasSuffix(name, "/internal/backup.ListGitSnapshots") || strings.HasSuffix(name, "/internal/backup.List"):
					addFinding(&findings, seen, "admin.operations.backups", filepath.ToSlash(path), name, "touches backup operations", "direct")
				case strings.HasSuffix(name, "/internal/updater.Check") || strings.HasSuffix(name, "/internal/updater.ScheduleApply"):
					addFinding(&findings, seen, "admin.operations.updates", filepath.ToSlash(path), name, "touches update operations", "direct")
				}
				return true
			})
		}
		return nil
	})
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Evidence < findings[j].Evidence
	})
	return findings
}

func addFinding(findings *[]SecurityFinding, seen map[string]struct{}, category, path, evidence, message, evidenceType string) {
	key := category + "|" + path + "|" + evidence
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*findings = append(*findings, SecurityFinding{
		Category:     category,
		Path:         path,
		Evidence:     evidence,
		Message:      message,
		EvidenceType: evidenceType,
	})
}

func compareDeclaredPermissions(meta Metadata, findings []SecurityFinding) []ValidationDiagnostic {
	out := []ValidationDiagnostic{}
	add := func(category, evidence, message string) {
		out = append(out, ValidationDiagnostic{
			Severity: "error",
			Path:     filepath.ToSlash(filepath.Join(meta.Directory, "plugin.yaml")),
			Message:  fmt.Sprintf("%s (%s): %s", category, evidence, message),
		})
	}

	for _, finding := range findings {
		switch finding.Category {
		case "filesystem.read":
			if !meta.Permissions.Filesystem.Read.Content && !meta.Permissions.Filesystem.Read.Data && !meta.Permissions.Filesystem.Read.Public && !meta.Permissions.Filesystem.Read.Themes && !meta.Permissions.Filesystem.Read.Plugins && !meta.Permissions.Filesystem.Read.Config && len(meta.Permissions.Filesystem.Read.Custom) == 0 {
				add(finding.Category, finding.Evidence, "filesystem read not declared in permissions.filesystem.read")
			}
		case "filesystem.write":
			if !meta.Permissions.Filesystem.Write.Content && !meta.Permissions.Filesystem.Write.Data && !meta.Permissions.Filesystem.Write.Public && !meta.Permissions.Filesystem.Write.Cache && !meta.Permissions.Filesystem.Write.Backups && len(meta.Permissions.Filesystem.Write.Custom) == 0 {
				add(finding.Category, finding.Evidence, "filesystem write not declared in permissions.filesystem.write")
			}
		case "filesystem.delete":
			if !meta.Permissions.Filesystem.Delete.Content && !meta.Permissions.Filesystem.Delete.Data && !meta.Permissions.Filesystem.Delete.Public && !meta.Permissions.Filesystem.Delete.Cache && !meta.Permissions.Filesystem.Delete.Backups && len(meta.Permissions.Filesystem.Delete.Custom) == 0 {
				add(finding.Category, finding.Evidence, "filesystem delete not declared in permissions.filesystem.delete")
			}
		case "environment.read":
			if !meta.Permissions.Environment.Read.Allowed {
				add(finding.Category, finding.Evidence, "environment access not declared in permissions.environment.read")
			}
		case "network.outbound":
			if !meta.Permissions.Network.Outbound.HTTP && !meta.Permissions.Network.Outbound.HTTPS && !meta.Permissions.Network.Outbound.WebSocket && !meta.Permissions.Network.Outbound.GRPC && len(meta.Permissions.Network.Outbound.CustomSchemes) == 0 {
				add(finding.Category, finding.Evidence, "outbound network access not declared in permissions.network.outbound")
			}
		case "network.inbound.register_routes":
			if !meta.Permissions.Network.Inbound.RegisterRoutes {
				add(finding.Category, finding.Evidence, "route registration not declared in permissions.network.inbound.register_routes")
			}
		case "process.exec":
			if !meta.Permissions.Process.Exec.Allowed {
				add(finding.Category, finding.Evidence, "process execution not declared in permissions.process.exec")
			}
		case "process.shell":
			if !meta.Permissions.Process.Shell.Allowed {
				add(finding.Category, finding.Evidence, "shell execution not declared in permissions.process.shell")
			}
		case "render.context":
			if !meta.Permissions.Render.Context.Read && !meta.Permissions.Render.Context.Write {
				add(finding.Category, finding.Evidence, "render context access not declared in permissions.render.context")
			}
		case "render.assets":
			if !meta.Permissions.Render.Assets.InjectCSS && !meta.Permissions.Render.Assets.InjectJS && !meta.Permissions.Render.Assets.InjectRemoteAssets {
				add(finding.Category, finding.Evidence, "asset injection not declared in permissions.render.assets")
			}
		case "render.html_slots":
			if !meta.Permissions.Render.HTMLSlots.Inject {
				add(finding.Category, finding.Evidence, "slot injection not declared in permissions.render.html_slots")
			}
		case "render.after_render":
			if !meta.Permissions.Render.AfterRender.MutateHTML {
				add(finding.Category, finding.Evidence, "after-render mutation not declared in permissions.render.after_render")
			}
		case "graph.read":
			if !meta.Permissions.Graph.Read && !meta.Permissions.Graph.Mutate {
				add(finding.Category, finding.Evidence, "graph access not declared in permissions.graph")
			}
		case "graph.mutate":
			if !meta.Permissions.Graph.Mutate && !meta.Permissions.Graph.Routes.Mutate {
				add(finding.Category, finding.Evidence, "graph/route mutation not declared in permissions.graph")
			}
		case "graph.taxonomies.inspect":
			if !meta.Permissions.Graph.Taxonomies.Inspect && !meta.Permissions.Graph.Read {
				add(finding.Category, finding.Evidence, "taxonomy graph access not declared in permissions.graph.taxonomies.inspect")
			}
		case "content.documents.read":
			if !meta.Permissions.Content.Documents.Read {
				add(finding.Category, finding.Evidence, "document access not declared in permissions.content.documents.read")
			}
		case "runtime.server.on_started":
			if !meta.Permissions.Runtime.Server.OnStarted {
				add(finding.Category, finding.Evidence, "server-start hook not declared in permissions.runtime.server.on_started")
			}
		case "capabilities.dangerous":
			if !meta.Permissions.Capabilities.Dangerous {
				add(finding.Category, finding.Evidence, "dangerous capability not declared in permissions.capabilities.dangerous")
			}
		case "capabilities.dynamic_loading":
			if !meta.Permissions.Capabilities.Dangerous {
				add(finding.Category, finding.Evidence, "dynamic plugin loading must declare permissions.capabilities.dangerous")
			}
		case "secrets.path_access":
			if !meta.Permissions.Secrets.Access.EnvSecrets && !meta.Permissions.Secrets.Access.DeployKeys && !meta.Permissions.Secrets.Access.SessionStore && !meta.Permissions.Secrets.Access.UpdateCredentials {
				add(finding.Category, finding.Evidence, "secret-looking file access must declare matching permissions.secrets.access capability")
			}
		case "admin.operations.backups":
			if !meta.Permissions.Admin.Operations.Backups {
				add(finding.Category, finding.Evidence, "backup operations not declared in permissions.admin.operations.backups")
			}
		case "admin.operations.updates":
			if !meta.Permissions.Admin.Operations.Updates {
				add(finding.Category, finding.Evidence, "update operations not declared in permissions.admin.operations.updates")
			}
		case "admin.operations.rebuild":
			if !meta.Permissions.Admin.Operations.Rebuild {
				add(finding.Category, finding.Evidence, "rebuild operations not declared in permissions.admin.operations.rebuild")
			}
		case "admin.operations.clear_cache":
			if !meta.Permissions.Admin.Operations.ClearCache {
				add(finding.Category, finding.Evidence, "cache clear operations not declared in permissions.admin.operations.clear_cache")
			}
		case "admin.audit.read":
			if !meta.Permissions.Admin.Audit.Read {
				add(finding.Category, finding.Evidence, "audit reads not declared in permissions.admin.audit.read")
			}
		case "admin.users.read":
			if !meta.Permissions.Admin.Users.Read {
				add(finding.Category, finding.Evidence, "admin user reads not declared in permissions.admin.users.read")
			}
		case "admin.users.write":
			if !meta.Permissions.Admin.Users.Write {
				add(finding.Category, finding.Evidence, "admin user writes not declared in permissions.admin.users.write")
			}
		case "admin.users.revoke_sessions":
			if !meta.Permissions.Admin.Users.RevokeSessions {
				add(finding.Category, finding.Evidence, "session revocation not declared in permissions.admin.users.revoke_sessions")
			}
		case "admin.users.reset_passwords":
			if !meta.Permissions.Admin.Users.ResetPasswords {
				add(finding.Category, finding.Evidence, "password reset operations not declared in permissions.admin.users.reset_passwords")
			}
		}
	}

	if len(meta.AdminExtensions.Pages) > 0 && !meta.Permissions.Admin.Extensions.Pages {
		add("admin.extensions.pages", "plugin.yaml:admin.pages", "admin page extensions not declared in permissions.admin.extensions.pages")
	}
	if len(meta.AdminExtensions.Widgets) > 0 && !meta.Permissions.Admin.Extensions.Widgets {
		add("admin.extensions.widgets", "plugin.yaml:admin.widgets", "admin widgets not declared in permissions.admin.extensions.widgets")
	}
	if len(meta.AdminExtensions.SettingsSections) > 0 && !meta.Permissions.Admin.Extensions.SettingsSections {
		add("admin.extensions.settings_sections", "plugin.yaml:admin.settings_sections", "admin settings sections not declared in permissions.admin.extensions.settings_sections")
	}
	if len(meta.AdminExtensions.Slots) > 0 && !meta.Permissions.Admin.Extensions.Slots {
		add("admin.extensions.slots", "plugin.yaml:admin.slots", "admin slots not declared in permissions.admin.extensions.slots")
	}

	return out
}

func SecurityApprovalRequired(meta Metadata, report SecurityReport) bool {
	return report.RequiresApproval || len(report.Mismatches) > 0 || strings.EqualFold(meta.Runtime.Mode, "rpc")
}

func capabilityBoundaryForRuntime(runtime RuntimeConfig) []string {
	mode := strings.ToLower(strings.TrimSpace(runtime.Mode))
	if mode == "rpc" {
		return []string{
			"host-to-plugin messages only expose declared hook payloads",
			"plugin process receives sanitized environment only",
			"host does not expose direct config, session, or filesystem channels",
		}
	}
	return []string{
		"in-process plugin shares Foundry process memory",
		"declared permissions are advisory and validated, not OS-isolated",
	}
}

func looksLikeSecretPath(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	candidates := []string{".env", "id_rsa", "credentials", "secrets", "token", "session", "passwd", "private_key"}
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
