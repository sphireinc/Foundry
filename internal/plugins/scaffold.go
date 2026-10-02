package plugins

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var authorPluginName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)

// Scaffold creates a disabled plugin starter without running code or changing
// configuration. Compiled starters belong inside the Foundry source module;
// RPC starters are independent modules using the public protocol SDK.
func Scaffold(pluginsDir, name, runtime string) (string, error) {
	if !authorPluginName.MatchString(name) {
		return "", fmt.Errorf("plugin name must start with a lowercase letter and contain lowercase letters, digits, hyphens or underscores")
	}
	if runtime != "rpc" && runtime != "compiled" {
		return "", fmt.Errorf("runtime must be rpc or compiled")
	}
	root := filepath.Join(pluginsDir, name)
	if err := os.MkdirAll(pluginsDir, 0o750); err != nil {
		return "", err
	}
	if err := os.Mkdir(root, 0o750); err != nil {
		return "", fmt.Errorf("create plugin starter (existing paths are never overwritten): %w", err)
	}
	files := map[string]string{"plugin.yaml": pluginStarterManifest(name, runtime), "README.md": pluginStarterReadme(name, runtime), ".gitignore": "/bin/\n"}
	source, test, sourcePath := compiledStarter, compiledStarterTest, "plugin.go"
	if runtime == "rpc" {
		source, test, sourcePath = rpcStarter, rpcStarterTest, "cmd/server/main.go"
		files["go.mod"] = "module example.com/foundry-plugins/" + name + "\n\ngo 1.26.0\n\nrequire github.com/sphireinc/foundry v1.4.6\n"
	}
	replacements := strings.NewReplacer("PLUGIN_NAME", name, "PACKAGE_NAME", "plugin_"+strings.ReplaceAll(name, "-", "_"), "CONTEXT_KEY", strings.ReplaceAll(name, "-", "_"))
	for path, body := range map[string]string{sourcePath: source, strings.TrimSuffix(sourcePath, ".go") + "_test.go": test} {
		formatted, err := format.Source([]byte(replacements.Replace(body)))
		if err != nil {
			return "", err
		}
		files[path] = string(formatted)
	}
	for path, body := range files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			return "", err
		}
	}
	return root, nil
}

func pluginStarterManifest(name, runtime string) string {
	manifest := fmt.Sprintf(`name: %s
title: %s
version: 0.1.0
description: Adds a word count to the page render context.
author: Unknown
license: MIT
foundry_api: v1
min_foundry_version: 1.4.6
compatibility_version: v1
permissions:
  content:
    documents:
      read: true
  render:
    context:
      read: true
      write: true
`, name, name)
	if runtime == "rpc" {
		manifest += fmt.Sprintf(`  capabilities:
    requires_admin_approval: true
runtime:
  mode: rpc
  protocol_version: v1alpha1
  command: ["./bin/%s"]
  sandbox:
    profile: default
    allow_network: false
    allow_filesystem_write: false
    allow_process_exec: false
`, name)
	}
	return manifest
}

func pluginStarterReadme(name, runtime string) string {
	common := fmt.Sprintf("# %s\n\nAdds a word count under `.Data.%s.words`. No network or filesystem access is needed by the hook.\n\n", name, strings.ReplaceAll(name, "-", "_"))
	if runtime == "rpc" {
		common += fmt.Sprintf("From this plugin directory (Go 1.26 or newer):\n\n```sh\ngo mod tidy\ngo test ./...\nmkdir -p bin\ngo build -o bin/%s ./cmd/server\n```\n\nThe manifest runs the built binary, not `go run`. Rebuild it after source changes, then restart Foundry. Keep stdout reserved for JSON-RPC; send diagnostics to stderr.\n\n", name)
	} else {
		common += fmt.Sprintf("Keep this directory inside `plugins/` in a Foundry source checkout; compiled plugins import Foundry internal packages and cannot be built as standalone external modules. From the checkout root:\n\n```sh\ngo test ./plugins/%s\nfoundry plugin validate %s --security\nfoundry plugin enable %s\nfoundry plugin sync\ngo build -o ./foundry ./cmd/foundry\n```\n\nRestart the rebuilt Foundry binary after changing source. Compiled plugins run with the host process's privileges.\n\n", name, name, name)
	}
	common += fmt.Sprintf("From the site root, run `foundry plugin validate %s --security` before enabling. Validation checks metadata and detected permissions; it does not compile, launch, or prove that the plugin is safe. Enabling may require `--approve-risk` after inspecting `foundry plugin security %s`. Initializing this starter does not enable it.\n\nThe RPC default sandbox is a trusted development mode, not an OS isolation guarantee. For deployment, inspect the strict-profile requirements in the plugin documentation.\n", name, name)
	return common
}

const compiledStarter = `package PACKAGE_NAME
import (
 "strings"
 "github.com/sphireinc/foundry/internal/plugins"
 "github.com/sphireinc/foundry/internal/renderer"
)
type Plugin struct{}
func (*Plugin) Name() string { return "PLUGIN_NAME" }
func (*Plugin) OnContext(ctx *renderer.ViewData) error {
 words := 0
 if ctx.Page != nil { words = len(strings.Fields(ctx.Page.RawBody)) }
 if ctx.Data == nil { ctx.Data = map[string]any{} }
 ctx.Data["CONTEXT_KEY"] = map[string]any{"words": words}
 return nil
}
func init() { plugins.Register("PLUGIN_NAME", func() plugins.Plugin { return &Plugin{} }) }
`
const compiledStarterTest = `package PACKAGE_NAME
import (
 "testing"
 "github.com/sphireinc/foundry/internal/content"
 "github.com/sphireinc/foundry/internal/renderer"
)
func TestWordCount(t *testing.T) {
 for _, tc := range []struct{page *content.Document; words int}{{nil,0},{&content.Document{RawBody:"one two three"},3}} {
 ctx := &renderer.ViewData{Page:tc.page}
 if err := (&Plugin{}).OnContext(ctx); err != nil { t.Fatal(err) }
 if got := ctx.Data["CONTEXT_KEY"].(map[string]any)["words"]; got != tc.words { t.Fatalf("got %v want %d",got,tc.words) }
 }
}
`
const rpcStarter = `package main
import (
 "fmt"
 "os"
 "strings"
 "github.com/sphireinc/foundry/sdk/pluginrpc"
)
type handler struct{}
func (handler) Handshake(req pluginrpc.HandshakeRequest) (pluginrpc.HandshakeResponse,error) {
 return pluginrpc.HandshakeResponse{PluginName:req.PluginName,ProtocolVersion:req.ProtocolVersion,SupportedHooks:[]string{pluginrpc.MethodContext}},nil
}
func (handler) Context(req pluginrpc.ContextRequest) (pluginrpc.ContextResponse,error) {
 words := 0
 if req.Page != nil { words = len(strings.Fields(req.Page.RawBody)) }
 return pluginrpc.ContextResponse{Data:map[string]any{"CONTEXT_KEY":map[string]any{"words":words}}},nil
}
func (handler) Shutdown() error { return nil }
func main() {
 server := pluginrpc.Server{Reader:os.Stdin,Writer:os.Stdout}
 if err := server.Serve(handler{}); err != nil { _, _ = fmt.Fprintln(os.Stderr,err); os.Exit(1) }
}
`
const rpcStarterTest = `package main
import (
 "testing"
 "github.com/sphireinc/foundry/sdk/pluginrpc"
)
func TestWordCount(t *testing.T) {
 for _, tc := range []struct{page *pluginrpc.PagePayload; words int}{{nil,0},{&pluginrpc.PagePayload{RawBody:"one two three"},3}} {
 result,err := (handler{}).Context(pluginrpc.ContextRequest{Page:tc.page})
 if err != nil { t.Fatal(err) }
 if got := result.Data["CONTEXT_KEY"].(map[string]any)["words"]; got != tc.words { t.Fatalf("got %v want %d",got,tc.words) }
 }
}
`
