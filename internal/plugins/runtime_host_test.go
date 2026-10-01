package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/sdk/pluginrpc"
)

// Run the test executable as an independent RPC peer, without a source runner.
func TestRPCPeer(t *testing.T) {
	args := os.Args
	marker := -1
	for i, arg := range args {
		if arg == "--rpc-peer" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	mode := args[marker+1]
	if mode == "heartbeat" {
		for i := 0; i < 100; i++ {
			_ = os.WriteFile(args[marker+2], []byte(fmt.Sprint(i)), 0o600) // #nosec G304 G703 -- test-owned heartbeat used to verify descendant cleanup.
			time.Sleep(50 * time.Millisecond)
		}
		os.Exit(0)
	}
	var child *exec.Cmd
	dec, enc := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var req pluginrpc.Request
		if dec.Decode(&req) != nil {
			os.Exit(0)
		}
		var result any
		switch req.Method {
		case pluginrpc.MethodHandshake:
			if mode == "hang" {
				time.Sleep(time.Minute)
			}
			name := "peer"
			if mode == "identity" {
				name = "imposter"
			}
			result = pluginrpc.HandshakeResponse{PluginName: name, ProtocolVersion: "v1alpha1", SupportedHooks: []string{pluginrpc.MethodContext, pluginrpc.MethodHTMLSlots, pluginrpc.MethodAfterRender}}
		case pluginrpc.MethodContext:
			var body pluginrpc.ContextRequest
			if err := json.Unmarshal(req.Params, &body); err != nil {
				os.Exit(2)
			}
			data := map[string]any{"saw_page": body.Page != nil, "saw_data": len(body.Data) > 0, "saw_title": body.Title != "", "saw_path": body.RequestPath != "", "saw_lang": body.Lang != ""}
			if mode == "child" && child == nil {
				child = exec.Command(args[0], "-test.run=^TestRPCPeer$", "--", "--rpc-peer", "heartbeat", args[marker+2]) // #nosec G204 G702 -- launches this test executable to verify process-group cleanup.
				if err := child.Start(); err != nil {
					os.Exit(2)
				}
			}
			if mode == "strict" {
				_, readErr := os.ReadFile(args[marker+2])                                                                 // #nosec G304 G703 -- negative sandbox probe intentionally reads an externally supplied test path.
				_, aliasReadErr := os.ReadFile("/System/Volumes/Data" + args[marker+2])                                   // #nosec G304 G703 -- negative test probes the macOS Data-volume alias of the same external file.
				writeErr := os.WriteFile(filepath.Join(os.Getenv("FOUNDRY_PLUGIN_DIR"), "forbidden"), []byte("x"), 0o600) // #nosec G304 G703 -- negative sandbox probe intentionally attempts a test-directory write.
				conn, networkErr := net.DialTimeout("tcp", "127.0.0.1:9", time.Second)
				if conn != nil {
					_ = conn.Close()
				}
				processErr := exec.Command("/bin/echo", "forbidden").Run()
				data["read_denied"] = errors.Is(readErr, os.ErrPermission)
				data["alias_read_denied"] = errors.Is(aliasReadErr, os.ErrPermission)
				data["write_denied"] = errors.Is(writeErr, os.ErrPermission)
				data["network_denied"] = errors.Is(networkErr, os.ErrPermission)
				data["process_denied"] = errors.Is(processErr, os.ErrPermission)
			}
			result = pluginrpc.ContextResponse{Data: data}
		case pluginrpc.MethodHTMLSlots:
			result = pluginrpc.HTMLSlotsResponse{Slots: map[string][]string{"head": {"<meta name=rpc>"}}}
		case pluginrpc.MethodAfterRender:
			var body pluginrpc.AfterRenderRequest
			if err := json.Unmarshal(req.Params, &body); err != nil {
				os.Exit(2)
			}
			result = pluginrpc.AfterRenderResponse{HTML: body.HTML + "<!-- rpc -->"}
		}
		body, err := json.Marshal(result)
		if err != nil {
			os.Exit(2)
		}
		id := req.ID
		if mode == "id" {
			id++
		}
		if mode == "oversize" && req.Method == pluginrpc.MethodContext {
			_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("x", 9*1024*1024))
			continue
		}
		if enc.Encode(pluginrpc.Response{ID: id, Result: body}) != nil {
			os.Exit(0)
		}
	}
}

func peerProxy(t *testing.T, mode string) *rpcPluginProxy {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	meta := Metadata{Name: "peer", Directory: t.TempDir(), Runtime: RuntimeConfig{Mode: "rpc", ProtocolVersion: "v1alpha1", Command: []string{executable, "-test.run=^TestRPCPeer$", "--", "--rpc-peer", mode}}}
	meta.Permissions.Render.Context.Read = true
	meta.Permissions.Render.Context.Write = true
	p, err := newRPCPluginProxy(meta)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.client.mu.Lock(); defer p.client.mu.Unlock(); p.client.stopLocked() })
	return p
}

func TestRPCPermissionsAndHooks(t *testing.T) {
	p := peerProxy(t, "normal")
	view := &renderer.ViewData{Title: "private", Lang: "en", RequestPath: "/private", Page: &content.Document{Title: "private"}, Data: map[string]any{"private": "value"}}
	p.meta.Permissions.Render.Context.Read = false
	if err := p.OnContext(view); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"saw_page", "saw_data", "saw_title", "saw_path", "saw_lang"} {
		if view.Data[key] != false {
			t.Fatalf("undeclared read %s leaked: %#v", key, view.Data)
		}
	}
	p.meta.Permissions.Render.Context.Write = false
	view.Data = map[string]any{"original": true}
	if err := p.OnContext(view); err != nil {
		t.Fatal(err)
	}
	if len(view.Data) != 1 {
		t.Fatal("undeclared mutation applied")
	}
	p.meta.Permissions.Render.Context.Read = true
	if err := p.OnContext(view); err != nil {
		t.Fatal(err)
	}
	if len(view.Data) != 1 {
		t.Fatal("read-only context hook mutated host data")
	}
	slots := renderer.NewSlots()
	if err := p.OnHTMLSlots(view, slots); err != nil {
		t.Fatal(err)
	}
	if slots.Render("head") != "" {
		t.Fatal("undeclared slot injection applied")
	}
	html, err := p.OnAfterRender("/", []byte("original"))
	if err != nil || string(html) != "original" {
		t.Fatalf("undeclared mutation: %s %v", html, err)
	}
	// Refresh the negotiated hook set after changing this test's declarations.
	p.client.stopLocked()
	p.meta.Permissions.Render.HTMLSlots.Inject = true
	p.meta.Permissions.Render.AfterRender.MutateHTML = true
	p.client.meta = p.meta
	if err := p.OnHTMLSlots(view, slots); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(slots.Render("head")), "<meta name=rpc>") {
		t.Fatal("declared slot missing")
	}
	html, err = p.OnAfterRender("/", []byte("original"))
	if err != nil || string(html) != "original<!-- rpc -->" {
		t.Fatalf("declared mutation: %s %v", html, err)
	}
}

func TestRPCRejectsInvalidPeers(t *testing.T) {
	for _, mode := range []string{"identity", "id", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			p := peerProxy(t, mode)
			if err := p.OnContext(&renderer.ViewData{}); err == nil {
				t.Fatal("invalid peer accepted")
			}
			if p.client.cmd != nil {
				t.Fatal("invalid peer not stopped")
			}
		})
	}
}

func TestRPCTimeoutAndClose(t *testing.T) {
	p := peerProxy(t, "hang")
	start := time.Now()
	if err := p.OnContext(&renderer.ViewData{}); err == nil {
		t.Fatal("hung peer accepted")
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("deadline exceeded: %s", elapsed)
	}
	if p.client.cmd != nil {
		t.Fatal("timed out peer not reaped")
	}
	p = peerProxy(t, "normal")
	if err := p.OnContext(&renderer.ViewData{}); err != nil {
		t.Fatal(err)
	}
	process := p.client.cmd
	manager := &Manager{plugins: []Plugin{p}}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if process.ProcessState == nil || p.client.cmd != nil {
		t.Fatal("manager close did not reap peer")
	}
	if err := manager.Close(); err != nil {
		t.Fatal("close must be idempotent")
	}
}

func TestStrictExecutableRejectsEscape(t *testing.T) {
	p := peerProxy(t, "normal")
	if _, _, err := strictExecutable(p.meta); err == nil {
		t.Fatal("external executable accepted")
	}
	path := filepath.Join(p.meta.Directory, "escape")
	if err := os.Symlink(p.meta.Runtime.Command[0], path); err != nil {
		t.Fatal(err)
	}
	p.meta.Runtime.Command[0] = path
	if _, _, err := strictExecutable(p.meta); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

func TestRPCCloseStopsDescendants(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("process-group cleanup is Unix-specific")
	}
	p := peerProxy(t, "child")
	heartbeat := filepath.Join(p.meta.Directory, "heartbeat")
	p.meta.Runtime.Command = append(p.meta.Runtime.Command, heartbeat)
	p.client.meta = p.meta
	if err := p.OnContext(&renderer.ViewData{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(heartbeat); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("descendant did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	manager := &Manager{plugins: []Plugin{p}}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	first, err := os.ReadFile(heartbeat) // #nosec G304 -- test-owned heartbeat in a temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	second, err := os.ReadFile(heartbeat) // #nosec G304 -- test-owned heartbeat in a temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("RPC child continued running after manager Close")
	}
}

func TestStrictRPCSandbox(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if strictSandboxAvailable() == nil {
			t.Fatal("unsupported platform must fail closed")
		}
		return
	}
	p := peerProxy(t, "strict")
	executable := p.meta.Runtime.Command[0]
	in, err := os.Open(executable) // #nosec G304 -- copies this running test executable, returned by os.Executable.
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	target := filepath.Join(p.meta.Directory, "peer")
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o700) // #nosec G304 G302 -- test-owned temporary executable needs its owner execute bit.
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy executable: %v %v", copyErr, closeErr)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err = filepath.EvalSymlinks(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("/System/Volumes/Data" + secret); err != nil {
		t.Fatalf("Data-volume probe must refer to an existing file: %v", err)
	}
	p.meta.Runtime.Command[0] = target
	p.meta.Runtime.Command = append(p.meta.Runtime.Command, secret)
	p.meta.Runtime.Sandbox.Profile = "strict"
	p.client.meta = p.meta
	view := &renderer.ViewData{}
	if err := p.OnContext(view); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"read_denied", "alias_read_denied", "write_denied", "network_denied", "process_denied"} {
		if view.Data[key] != true {
			t.Fatalf("strict sandbox did not enforce %s: %#v", key, view.Data)
		}
	}
}
