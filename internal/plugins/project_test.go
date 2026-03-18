package plugins

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

type testPlugin struct {
	name string
	log  *[]string
}

func (p *testPlugin) Name() string { return p.name }
func (p *testPlugin) OnConfigLoaded(*config.Config) error {
	*p.log = append(*p.log, "config")
	return nil
}
func (p *testPlugin) OnContentDiscovered(string) error {
	*p.log = append(*p.log, "discover")
	return nil
}
func (p *testPlugin) OnFrontmatterParsed(*content.Document) error {
	*p.log = append(*p.log, "frontmatter")
	return nil
}
func (p *testPlugin) OnMarkdownRendered(*content.Document) error {
	*p.log = append(*p.log, "markdown")
	return nil
}
func (p *testPlugin) OnDocumentParsed(*content.Document) error {
	*p.log = append(*p.log, "document")
	return nil
}
func (p *testPlugin) OnDataLoaded(map[string]any) error { *p.log = append(*p.log, "data"); return nil }
func (p *testPlugin) OnGraphBuilding(*content.SiteGraph) error {
	*p.log = append(*p.log, "graph-building")
	return nil
}
func (p *testPlugin) OnGraphBuilt(*content.SiteGraph) error {
	*p.log = append(*p.log, "graph-built")
	return nil
}
func (p *testPlugin) OnTaxonomyBuilt(*content.SiteGraph) error {
	*p.log = append(*p.log, "taxonomy")
	return nil
}
func (p *testPlugin) OnRoutesAssigned(*content.SiteGraph) error {
	*p.log = append(*p.log, "routes")
	return nil
}
func (p *testPlugin) OnContext(*renderer.ViewData) error {
	*p.log = append(*p.log, "context")
	return nil
}
func (p *testPlugin) OnAssets(*renderer.ViewData, *renderer.AssetSet) error {
	*p.log = append(*p.log, "assets")
	return nil
}
func (p *testPlugin) OnHTMLSlots(*renderer.ViewData, *renderer.Slots) error {
	*p.log = append(*p.log, "slots")
	return nil
}
func (p *testPlugin) OnBeforeRender(*renderer.ViewData) error {
	*p.log = append(*p.log, "before")
	return nil
}
func (p *testPlugin) OnAfterRender(_ string, html []byte) ([]byte, error) {
	*p.log = append(*p.log, "after")
	return append(html, byte('!')), nil
}
func (p *testPlugin) OnAssetsBuilding(*config.Config) error {
	*p.log = append(*p.log, "assets-building")
	return nil
}
func (p *testPlugin) OnBuildStarted() error { *p.log = append(*p.log, "build-started"); return nil }
func (p *testPlugin) OnBuildCompleted(*content.SiteGraph) error {
	*p.log = append(*p.log, "build-completed")
	return nil
}
func (p *testPlugin) OnServerStarted(string) error { *p.log = append(*p.log, "server"); return nil }
func (p *testPlugin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/x", func(http.ResponseWriter, *http.Request) {})
}
func (p *testPlugin) Commands() []Command {
	return []Command{{Name: "hello", Run: func(ctx CommandContext) error {
		_, _ = ctx.Stdout.Write([]byte("hi"))
		return nil
	}}}
}

func TestManagerHooksAndCLI(t *testing.T) {
	log := []string{}
	m := &Manager{
		plugins: []Plugin{&testPlugin{name: "test", log: &log}},
		metadata: map[string]Metadata{
			"test": {Name: "test"},
		},
	}

	graph := content.NewSiteGraph(&config.Config{})
	view := &renderer.ViewData{}
	slots := renderer.NewSlots()
	assets := renderer.NewAssetSet()

	if err := m.OnConfigLoaded(&config.Config{}); err != nil ||
		m.OnContentDiscovered("x") != nil ||
		m.OnFrontmatterParsed(&content.Document{}) != nil ||
		m.OnMarkdownRendered(&content.Document{}) != nil ||
		m.OnDocumentParsed(&content.Document{}) != nil ||
		m.OnDataLoaded(map[string]any{}) != nil ||
		m.OnGraphBuilding(graph) != nil ||
		m.OnGraphBuilt(graph) != nil ||
		m.OnTaxonomyBuilt(graph) != nil ||
		m.OnRoutesAssigned(graph) != nil ||
		m.OnContext(view) != nil ||
		m.OnAssets(view, assets) != nil ||
		m.OnHTMLSlots(view, slots) != nil ||
		m.OnBeforeRender(view) != nil ||
		m.OnAssetsBuilding(&config.Config{}) != nil ||
		m.OnBuildStarted() != nil ||
		m.OnBuildCompleted(graph) != nil ||
		m.OnServerStarted("addr") != nil {
		t.Fatal("expected manager hooks to succeed")
	}

	out, err := m.OnAfterRender("/", []byte("ok"))
	if err != nil || string(out) != "ok!" {
		t.Fatalf("unexpected after render result: %q %v", string(out), err)
	}

	buf := &bytes.Buffer{}
	if err := m.RunCommand("hello", CommandContext{Stdout: buf}); err != nil || buf.String() != "hi" {
		t.Fatalf("unexpected run command result: %q %v", buf.String(), err)
	}
	if err := m.RunCommand("", CommandContext{}); err == nil {
		t.Fatal("expected empty plugin command name error")
	}
	if err := m.RunCommand("missing", CommandContext{}); err == nil {
		t.Fatal("expected missing plugin command error")
	}
	if len(m.Commands()) != 1 || len(m.Plugins()) != 1 || len(m.Metadata()) != 1 || len(m.MetadataList()) != 1 {
		t.Fatal("expected manager accessors to return data")
	}
	if _, ok := m.MetadataFor("test"); !ok {
		t.Fatal("expected metadata lookup")
	}
}

func TestProjectDependencyStatusAndValidationHelpers(t *testing.T) {
	root := t.TempDir()
	writePluginMeta(t, root, "alpha", "github.com/acme/alpha", []string{"github.com/acme/beta"})
	writePluginMeta(t, root, "beta", "github.com/acme/beta", nil)
	writePluginCode(t, root, "alpha")
	writePluginCode(t, root, "beta")

	p := NewProject("config.yaml", root, "out.go", "example/module")
	meta, err := p.Metadata("alpha")
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	missing, err := p.MissingDependencies(meta, []string{"alpha"})
	if err != nil || len(missing) != 1 || !missing[0].Installed || missing[0].Name != "beta" {
		t.Fatalf("unexpected missing deps: %#v %v", missing, err)
	}
	statuses, err := p.DependencyStatuses("alpha", []string{"alpha"})
	if err != nil || len(statuses) != 1 || statuses[0].Status != "installed" {
		t.Fatalf("unexpected dependency statuses: %#v %v", statuses, err)
	}
	if report := p.ValidateEnabled([]string{"alpha", "beta"}); len(report.Passed) != 2 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	if statuses := p.EnabledStatuses([]string{"alpha"}); statuses["alpha"] != "enabled" {
		t.Fatalf("unexpected enabled status: %#v", statuses)
	}
	if installed, err := p.ListInstalled(); err != nil || len(installed) != 2 {
		t.Fatalf("unexpected installed plugins: %#v %v", installed, err)
	}
}

func TestProjectWrappersAndManagerHelpers(t *testing.T) {
	root := t.TempDir()
	writePluginMeta(t, root, "alpha", "github.com/acme/alpha", nil)
	writePluginCode(t, root, "alpha")

	p := NewProject(filepath.Join(root, "content", "config", "site.yaml"), root, filepath.Join(root, "internal", "generated", "plugins_gen.go"), "example/module")
	if err := os.MkdirAll(filepath.Join(root, "content", "config"), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(p.ConfigPath, []byte("plugins:\n  enabled:\n    - alpha\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := p.Sync(); err != nil {
		t.Fatalf("sync project: %v", err)
	}
	if err := p.Enable("alpha"); err != nil {
		t.Fatalf("enable project: %v", err)
	}
	if err := p.Disable("alpha"); err != nil {
		t.Fatalf("disable project: %v", err)
	}
	if err := p.Validate("alpha"); err != nil {
		t.Fatalf("validate project: %v", err)
	}
	if err := p.Uninstall("alpha"); err != nil {
		t.Fatalf("uninstall project: %v", err)
	}
	if _, err := p.Update("alpha"); err == nil {
		t.Fatal("expected update failure after uninstall")
	}
	if _, err := p.Install("", ""); err == nil {
		t.Fatal("expected install wrapper failure")
	}
}

func TestRegisterAndNewManager(t *testing.T) {
	name := "manager-test-plugin"
	Register(name, func() Plugin { return &testPlugin{name: name, log: new([]string)} })

	root := t.TempDir()
	writePluginMeta(t, root, name, "github.com/acme/"+name, nil)
	writePluginCode(t, root, name)

	m, err := NewManager(root, []string{name})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if len(m.Plugins()) != 1 {
		t.Fatalf("expected registered plugin instance, got %#v", m.Plugins())
	}

	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected registered route response, got %d", rr.Code)
	}

	writePluginMeta(t, root, "dup", "github.com/acme/"+name, nil)
	writePluginCode(t, root, "dup")
	if _, err := NewManager(root, []string{name, "dup"}); err == nil {
		t.Fatal("expected duplicate repo validation failure")
	}
}

func TestRegisterPanicsForInvalidCases(t *testing.T) {
	assertPanic := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		fn()
	}

	assertPanic(t, func() { Register("", func() Plugin { return &testPlugin{} }) })
	assertPanic(t, func() { Register("nil-factory-plugin", nil) })

	const dupName = "duplicate-registration-plugin"
	Register(dupName, func() Plugin { return &testPlugin{name: dupName, log: new([]string)} })
	assertPanic(t, func() { Register(dupName, func() Plugin { return &testPlugin{name: dupName, log: new([]string)} }) })
}

func writePluginMeta(t *testing.T, root, name, repo string, requires []string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	body := "name: " + name + "\nrepo: " + repo + "\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n"
	if len(requires) > 0 {
		body += "requires:\n"
		for _, req := range requires {
			body += "  - " + req + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write plugin metadata: %v", err)
	}
}

func writePluginCode(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name, "plugin.go"), []byte("package "+strings.ReplaceAll(name, "-", "")+"\n"), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}
}
