package theme

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/safepath"
)

// ValidationDiagnostic is a single frontend-theme validation finding.
type ValidationDiagnostic struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

// ValidationResult summarizes frontend-theme validation.
type ValidationResult struct {
	Valid       bool                   `json:"valid"`
	Diagnostics []ValidationDiagnostic `json:"diagnostics,omitempty"`
}

var templateReferencePattern = regexp.MustCompile(`{{\s*(?:template|block)\s+"([^"]+)"`)
var remoteURLPattern = regexp.MustCompile(`(?i)(https?|wss?)://[^\s"'()<>]+`)

// ValidateInstalledDetailed performs Foundry's full frontend-theme validation
// pass and returns all diagnostics.
//
// Validation checks manifest compatibility, required layouts and partials,
// required launch slots, template references, and template parse validity.
func ValidateInstalledDetailed(themesDir, name string) (*ValidationResult, error) {
	name, err := safepath.ValidatePathComponent("theme name", name)
	if err != nil {
		return nil, err
	}

	root := filepath.Join(themesDir, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("theme %q does not exist", name)
		}
		return nil, err
	}
	if symlinkInfo, lstatErr := os.Lstat(root); lstatErr == nil && symlinkInfo.Mode()&os.ModeSymlink != 0 {
		result := &ValidationResult{Valid: false, Diagnostics: []ValidationDiagnostic{{
			Severity: "error",
			Path:     filepath.ToSlash(root),
			Message:  "theme path is a symlink",
		}}}
		return result, nil
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("theme path %q is not a directory", root)
	}

	manifest, err := LoadManifest(themesDir, name)
	if err != nil {
		return nil, err
	}

	result := &ValidationResult{Valid: true, Diagnostics: make([]ValidationDiagnostic, 0)}
	add := func(severity, path, message string) {
		result.Diagnostics = append(result.Diagnostics, ValidationDiagnostic{
			Severity: severity,
			Path:     filepath.ToSlash(path),
			Message:  message,
		})
		if severity == "error" {
			result.Valid = false
		}
	}

	if strings.TrimSpace(manifest.Name) != name {
		add("error", filepath.Join(root, "theme.yaml"), fmt.Sprintf("theme manifest name %q must match directory %q", manifest.Name, name))
	}
	if manifest.SDKVersion != consts.FrontendSDKVersion {
		add("error", filepath.Join(root, "theme.yaml"), fmt.Sprintf("unsupported sdk_version %q", manifest.SDKVersion))
	}
	if manifest.CompatibilityVersion != consts.FrontendCompatibility {
		add("error", filepath.Join(root, "theme.yaml"), fmt.Sprintf("unsupported compatibility_version %q", manifest.CompatibilityVersion))
	}
	validateFieldContracts(filepath.Join(root, "theme.yaml"), manifest, add)

	requiredLayouts := manifest.RequiredLayouts()
	for _, layout := range requiredLayouts {
		path := filepath.Join(root, "layouts", layout+".html")
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			add("error", path, err.Error())
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				add("error", path, "missing required theme layout")
				continue
			}
			return nil, err
		}
	}

	requiredPartials := []string{
		filepath.Join(root, "layouts", "partials", "head.html"),
		filepath.Join(root, "layouts", "partials", "header.html"),
		filepath.Join(root, "layouts", "partials", "footer.html"),
	}
	for _, path := range requiredPartials {
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			add("error", path, err.Error())
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				add("error", path, "missing required theme partial")
				continue
			}
			return nil, err
		}
	}

	validateRequiredLaunchSlotsDetailed(root, manifest, add)
	validateFieldContractsDetailed(root, manifest, add)
	validateThemeSecurityDetailed(root, manifest, add)
	validateTemplateReferences(root, manifest, add)
	validateTemplateParsing(root, add)

	sort.Slice(result.Diagnostics, func(i, j int) bool {
		if result.Diagnostics[i].Severity != result.Diagnostics[j].Severity {
			return result.Diagnostics[i].Severity < result.Diagnostics[j].Severity
		}
		if result.Diagnostics[i].Path != result.Diagnostics[j].Path {
			return result.Diagnostics[i].Path < result.Diagnostics[j].Path
		}
		return result.Diagnostics[i].Message < result.Diagnostics[j].Message
	})

	return result, nil
}

func validateThemeSecurityDetailed(root string, manifest *Manifest, add func(severity, path, message string)) {
	keyPath := filepath.Join(root, "theme.yaml")
	normalizeThemeSecurity(&manifest.Security)

	if !manifest.Security.ExternalAssets.Allowed {
		for _, list := range [][]string{
			manifest.Security.ExternalAssets.Scripts,
			manifest.Security.ExternalAssets.Styles,
			manifest.Security.ExternalAssets.Fonts,
			manifest.Security.ExternalAssets.Images,
			manifest.Security.ExternalAssets.Media,
		} {
			if len(list) > 0 {
				add("error", keyPath, "security.external_assets.allowed must be true when remote asset allowlists are declared")
				break
			}
		}
	}
	if !manifest.Security.FrontendRequests.Allowed && len(manifest.Security.FrontendRequests.Origins) > 0 {
		add("error", keyPath, "security.frontend_requests.allowed must be true when remote request origins are declared")
	}
	if themeBoolValue(manifest.Security.TemplateContext.AllowRawConfig) {
		add("error", keyPath, "security.template_context.allow_raw_config is not supported; templates only receive a curated public-safe site config")
	}
	if themeBoolValue(manifest.Security.TemplateContext.AllowAdminState) {
		add("error", keyPath, "security.template_context.allow_admin_state is not supported; admin state is never exposed to themes")
	}
	if themeBoolValue(manifest.Security.TemplateContext.AllowRuntimeState) {
		add("warning", keyPath, "security.template_context.allow_runtime_state is advisory only; undeclared runtime/admin keys are still filtered from template data")
	}

	assetAllowlist := themeExternalAssetAllowlist(manifest.Security)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".html", ".css", ".js":
		default:
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		urls := remoteURLPattern.FindAllString(string(body), -1)
		for _, raw := range urls {
			switch ext {
			case ".js":
				if !manifest.Security.FrontendRequests.Allowed || !URLAllowedByPatterns(raw, manifest.Security.FrontendRequests.Origins) {
					add("error", path, fmt.Sprintf("remote frontend request %q is not declared in security.frontend_requests.origins", raw))
				}
			default:
				if !manifest.Security.ExternalAssets.Allowed || !URLAllowedByPatterns(raw, assetAllowlist) {
					add("error", path, fmt.Sprintf("remote asset %q is not declared in security.external_assets allowlists", raw))
				}
			}
		}
		return nil
	})
	if err != nil {
		add("error", root, err.Error())
	}
}

func validateFieldContracts(manifestPath string, manifest *Manifest, add func(severity, path, message string)) {
	root := filepath.Dir(manifestPath)
	validateFieldContractsDetailed(root, manifest, add)
}

func validateFieldContractsDetailed(root string, manifest *Manifest, add func(severity, path, message string)) {
	seen := make(map[string]struct{})
	for index, contract := range manifest.FieldContracts {
		keyPath := filepath.Join(root, "theme.yaml")
		key := strings.TrimSpace(contract.Key)
		if key == "" {
			add("error", keyPath, fmt.Sprintf("field_contracts[%d] must define key", index))
		} else {
			if _, ok := seen[key]; ok {
				add("error", keyPath, fmt.Sprintf("field_contracts[%d] key %q must be unique", index, key))
			}
			seen[key] = struct{}{}
		}
		scope := normalizeFieldContractScope(contract.Target.Scope)
		switch scope {
		case "document":
			if len(contract.Target.Types) == 0 && len(contract.Target.Layouts) == 0 && len(contract.Target.Slugs) == 0 {
				add("error", keyPath, fmt.Sprintf("field_contracts[%d] document target must declare at least one of types, layouts, or slugs", index))
			}
		case "shared":
			if strings.TrimSpace(contract.Target.Key) == "" {
				add("error", keyPath, fmt.Sprintf("field_contracts[%d] shared target must define target.key", index))
			}
		default:
			add("error", keyPath, fmt.Sprintf("field_contracts[%d] has unsupported target.scope %q", index, contract.Target.Scope))
		}
		if len(contract.Fields) == 0 {
			add("error", keyPath, fmt.Sprintf("field_contracts[%d] must define at least one field", index))
			continue
		}
		validateFieldDefinitions(keyPath, contract.Fields, add)
	}
}

func validateFieldDefinitions(path string, defs []foundryconfig.FieldDefinition, add func(severity, path, message string)) {
	seen := make(map[string]struct{})
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			add("error", path, "field definitions must have a name")
			continue
		}
		if _, ok := seen[name]; ok {
			add("error", path, fmt.Sprintf("field definition %q must be unique within its contract", name))
		}
		seen[name] = struct{}{}
		if strings.TrimSpace(def.Type) == "" {
			add("error", path, fmt.Sprintf("field %q must define a type", name))
		}
		switch strings.ToLower(strings.TrimSpace(def.Type)) {
		case "object":
			if len(def.Fields) == 0 {
				add("error", path, fmt.Sprintf("object field %q must define nested fields", name))
			} else {
				validateFieldDefinitions(path, def.Fields, add)
			}
		case "repeater", "list", "array":
			if def.Item == nil {
				add("error", path, fmt.Sprintf("repeater field %q must define item", name))
			} else {
				validateFieldDefinitions(path, []foundryconfig.FieldDefinition{*def.Item}, add)
			}
		}
	}
}

// validateRequiredLaunchSlotsDetailed enforces the slot contract Foundry expects
// launch-ready frontend themes to expose and actually render.
func validateRequiredLaunchSlotsDetailed(root string, manifest *Manifest, add func(severity, path, message string)) {
	declared := make(map[string]struct{}, len(manifest.Slots))
	for _, slot := range manifest.Slots {
		declared[strings.TrimSpace(slot)] = struct{}{}
	}

	for _, slot := range requiredLaunchSlots {
		if _, ok := declared[slot]; !ok {
			add("error", filepath.Join(root, "theme.yaml"), fmt.Sprintf("theme manifest must declare required launch slot %q", slot))
		}
	}

	for slot, relPath := range requiredLaunchSlotFiles {
		path := filepath.Join(root, relPath)
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			add("error", path, err.Error())
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				add("error", path, fmt.Sprintf("missing layout required to render slot %q", slot))
				continue
			}
			add("error", path, err.Error())
			continue
		}
		call := fmt.Sprintf(`pluginSlot %q`, slot)
		if !strings.Contains(string(body), call) {
			add("error", path, fmt.Sprintf("required launch slot %q is not rendered", slot))
		}
	}
}

// validateTemplateReferences checks that template and block calls only target
// known layouts or partials.
func validateTemplateReferences(root string, manifest *Manifest, add func(severity, path, message string)) {
	known := map[string]struct{}{
		"base":    {},
		"content": {},
	}
	for _, layout := range manifest.RequiredLayouts() {
		known[layout] = struct{}{}
	}
	partials, _ := filepath.Glob(filepath.Join(root, "layouts", "partials", "*.html"))
	for _, partial := range partials {
		name := strings.TrimSuffix(filepath.Base(partial), filepath.Ext(partial))
		known[name] = struct{}{}
	}

	files, _ := filepath.Glob(filepath.Join(root, "layouts", "*.html"))
	files = append(files, partials...)
	for _, path := range files {
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			add("error", path, err.Error())
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		matches := templateReferencePattern.FindAllStringSubmatch(string(body), -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			if _, ok := known[name]; !ok {
				add("error", path, fmt.Sprintf("template references unknown partial or layout %q", name))
			}
		}
	}
}

// validateTemplateParsing parses the theme templates with Foundry's supported
// helper functions to catch syntax errors early.
func validateTemplateParsing(root string, add func(severity, path, message string)) {
	partials, _ := filepath.Glob(filepath.Join(root, "layouts", "partials", "*.html"))
	files, _ := filepath.Glob(filepath.Join(root, "layouts", "*.html"))
	files = append(files, partials...)
	if len(files) == 0 {
		return
	}
	for _, path := range files {
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			add("error", path, err.Error())
			return
		}
	}
	_, err := template.New("validate").Funcs(template.FuncMap{
		"safeHTML": func(v any) template.HTML { return "" },
		"field":    func(any, string) any { return nil },
		"data":     func(string) any { return nil },
		"pluginSlot": func(string) template.HTML {
			return ""
		},
	}).ParseFiles(files...)
	if err != nil {
		add("error", filepath.Join(root, "layouts"), fmt.Sprintf("invalid template parse: %v", err))
	}
}
