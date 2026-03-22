package theme

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/safepath"
)

type ValidationDiagnostic struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type ValidationResult struct {
	Valid       bool                   `json:"valid"`
	Diagnostics []ValidationDiagnostic `json:"diagnostics,omitempty"`
}

var templateReferencePattern = regexp.MustCompile(`{{\s*(?:template|block)\s+"([^"]+)"`)

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

	requiredLayouts := manifest.RequiredLayouts()
	for _, layout := range requiredLayouts {
		path := filepath.Join(root, "layouts", layout+".html")
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
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				add("error", path, "missing required theme partial")
				continue
			}
			return nil, err
		}
	}

	validateRequiredLaunchSlotsDetailed(root, manifest, add)
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

func validateTemplateParsing(root string, add func(severity, path, message string)) {
	partials, _ := filepath.Glob(filepath.Join(root, "layouts", "partials", "*.html"))
	files, _ := filepath.Glob(filepath.Join(root, "layouts", "*.html"))
	files = append(files, partials...)
	if len(files) == 0 {
		return
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
