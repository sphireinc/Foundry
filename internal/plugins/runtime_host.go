package plugins

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/sdk/pluginrpc"
)

// RuntimeHost describes the execution host for a plugin runtime mode.
type RuntimeHost interface {
	Name() string
	Supports(meta Metadata) bool
}

type InProcessHost struct{}

func (InProcessHost) Name() string { return "in_process" }

func (InProcessHost) Supports(meta Metadata) bool {
	return stringsEqualFoldEmpty(strings.TrimSpace(meta.Runtime.Mode), "in_process")
}

type RPCHost struct{}

func (RPCHost) Name() string { return "rpc" }

func (RPCHost) Supports(meta Metadata) bool {
	return stringsEqualFoldEmpty(strings.TrimSpace(meta.Runtime.Mode), "rpc")
}

func ResolveRuntimeHost(meta Metadata) RuntimeHost {
	if stringsEqualFoldEmpty(strings.TrimSpace(meta.Runtime.Mode), "rpc") {
		return RPCHost{}
	}
	return InProcessHost{}
}

func EnsureRuntimeSupported(meta Metadata) error {
	mode := strings.ToLower(strings.TrimSpace(meta.Runtime.Mode))
	if mode == "" || mode == "in_process" {
		return nil
	}
	if mode != "rpc" {
		return fmt.Errorf("plugin %q declares unsupported runtime.mode=%q", meta.Name, meta.Runtime.Mode)
	}
	if len(meta.Runtime.Command) == 0 {
		return fmt.Errorf("plugin %q declares runtime.mode=rpc but runtime.command is empty", meta.Name)
	}
	if strings.TrimSpace(meta.Runtime.ProtocolVersion) == "" {
		return fmt.Errorf("plugin %q declares runtime.mode=rpc but runtime.protocol_version is empty", meta.Name)
	}
	if meta.Runtime.Sandbox.AllowNetwork {
		return fmt.Errorf("plugin %q declares runtime.mode=rpc with sandbox.allow_network=true, which is not supported by the current RPC host", meta.Name)
	}
	if meta.Runtime.Sandbox.AllowFilesystemWrite {
		return fmt.Errorf("plugin %q declares runtime.mode=rpc with sandbox.allow_filesystem_write=true, which is not supported by the current RPC host", meta.Name)
	}
	if meta.Runtime.Sandbox.AllowProcessExec {
		return fmt.Errorf("plugin %q declares runtime.mode=rpc with sandbox.allow_process_exec=true, which is not supported by the current RPC host", meta.Name)
	}
	if meta.Runtime.ProtocolVersion != "v1alpha1" {
		return fmt.Errorf("plugin %q declares unsupported RPC protocol %q", meta.Name, meta.Runtime.ProtocolVersion)
	}
	if meta.Runtime.Sandbox.Profile == "strict" {
		if err := strictSandboxAvailable(); err != nil {
			return err
		}
		if _, _, err := strictExecutable(meta); err != nil {
			return err
		}
	}
	return nil
}

func stringsEqualFoldEmpty(v, want string) bool {
	if v == "" {
		v = "in_process"
	}
	return strings.EqualFold(v, want)
}

type rpcPluginProxy struct {
	meta   Metadata
	client *rpcPluginClient
}

func newRPCPluginProxy(meta Metadata) (*rpcPluginProxy, error) {
	if err := EnsureRuntimeSupported(meta); err != nil {
		return nil, err
	}
	return &rpcPluginProxy{
		meta:   meta,
		client: &rpcPluginClient{meta: meta},
	}, nil
}

func (p *rpcPluginProxy) Name() string { return p.meta.Name }

func (p *rpcPluginProxy) OnContext(ctx *renderer.ViewData) error {
	if ctx == nil {
		return nil
	}
	permissions := p.meta.Permissions
	if !permissions.Render.Context.Read && !permissions.Render.Context.Write {
		return nil
	}
	req := p.contextRequest(ctx)
	resp, err := p.client.Context(req)
	if err != nil {
		return err
	}
	if !permissions.Render.Context.Write || len(resp.Data) == 0 {
		return nil
	}
	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}
	for key, value := range resp.Data {
		ctx.Data[key] = value
	}
	return nil
}

func (p *rpcPluginProxy) OnAfterRender(url string, html []byte) ([]byte, error) {
	if !p.meta.Permissions.Render.AfterRender.MutateHTML {
		return html, nil
	}
	p.client.mu.Lock()
	defer p.client.mu.Unlock()
	if err := p.client.ensureStartedLocked(); err != nil {
		return nil, err
	}
	if !hookSupported(p.client.handshake.SupportedHooks, pluginrpc.MethodAfterRender) {
		return html, nil
	}
	var response struct {
		HTML *string `json:"html"`
	}
	if err := p.client.callLocked(pluginrpc.MethodAfterRender, pluginrpc.AfterRenderRequest{URL: url, HTML: string(html)}, &response); err != nil {
		return nil, err
	}
	if response.HTML == nil {
		p.client.stopLocked()
		return nil, fmt.Errorf("rpc plugin %q omitted after-render HTML", p.meta.Name)
	}
	return []byte(*response.HTML), nil
}

func (p *rpcPluginProxy) OnHTMLSlots(view *renderer.ViewData, slots *renderer.Slots) error {
	if view == nil || slots == nil || !p.meta.Permissions.Render.HTMLSlots.Inject {
		return nil
	}
	p.client.mu.Lock()
	defer p.client.mu.Unlock()
	if err := p.client.ensureStartedLocked(); err != nil {
		return err
	}
	if !hookSupported(p.client.handshake.SupportedHooks, pluginrpc.MethodHTMLSlots) {
		return nil
	}
	req := p.contextRequest(view)
	var response pluginrpc.HTMLSlotsResponse
	if err := p.client.callLocked(pluginrpc.MethodHTMLSlots, req, &response); err != nil {
		return err
	}
	names := make([]string, 0, len(response.Slots))
	for name := range response.Slots {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, html := range response.Slots[name] {
			slots.Add(name, template.HTML(html)) // #nosec G203 -- explicit permissions.render.html_slots.inject grants raw HTML injection.
		}
	}
	return nil
}

func (p *rpcPluginProxy) contextRequest(view *renderer.ViewData) pluginrpc.ContextRequest {
	full := toRPCContextRequest(view)
	req := pluginrpc.ContextRequest{}
	if p.meta.Permissions.Render.Context.Read {
		req = full
	}
	req.Page = nil
	if p.meta.Permissions.Content.Documents.Read {
		req.Page = full.Page
	}
	return req
}

type rpcPluginClient struct {
	meta           Metadata
	mu             sync.Mutex
	cmd            *exec.Cmd
	input          io.WriteCloser
	output         io.ReadCloser
	stdin          *bufio.Writer
	encoder        *json.Encoder
	scanner        *bufio.Scanner
	nextID         int
	handshake      *pluginrpc.HandshakeResponse
	cancelDeadline func()
}

func (c *rpcPluginClient) Context(req pluginrpc.ContextRequest) (pluginrpc.ContextResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureStartedLocked(); err != nil {
		return pluginrpc.ContextResponse{}, err
	}
	if c.handshake == nil || !hookSupported(c.handshake.SupportedHooks, pluginrpc.MethodContext) {
		return pluginrpc.ContextResponse{}, nil
	}
	var resp pluginrpc.ContextResponse
	if err := c.callLocked(pluginrpc.MethodContext, req, &resp); err != nil {
		return pluginrpc.ContextResponse{}, err
	}
	return resp, nil
}

func (c *rpcPluginClient) ensureStartedLocked() error {
	if c.cmd != nil {
		return nil
	}
	cmd, err := rpcCommand(c.meta)
	if err != nil {
		return err
	}
	cmd.Dir = c.meta.Directory
	cmd.Env = c.rpcEnv()
	isolateRPCProcess(cmd)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.input = stdinPipe
	c.output = stdoutPipe
	c.stdin = bufio.NewWriter(stdinPipe)
	c.encoder = json.NewEncoder(c.stdin)
	c.scanner = bufio.NewScanner(stdoutPipe)
	c.scanner.Buffer(make([]byte, 4096), 8*1024*1024)

	var handshake pluginrpc.HandshakeResponse
	if err := c.callLocked(pluginrpc.MethodHandshake, pluginrpc.HandshakeRequest{
		PluginName:       c.meta.Name,
		ProtocolVersion:  c.meta.Runtime.ProtocolVersion,
		RequestedHooks:   c.requestedHooks(),
		SandboxProfile:   c.meta.Runtime.Sandbox.Profile,
		AllowNetwork:     c.meta.Runtime.Sandbox.AllowNetwork,
		AllowFSWrite:     c.meta.Runtime.Sandbox.AllowFilesystemWrite,
		AllowProcessExec: c.meta.Runtime.Sandbox.AllowProcessExec,
	}, &handshake); err != nil {
		c.stopLocked()
		return err
	}
	if handshake.PluginName != c.meta.Name || handshake.ProtocolVersion != c.meta.Runtime.ProtocolVersion {
		c.stopLocked()
		return fmt.Errorf("rpc plugin %q returned an incompatible handshake", c.meta.Name)
	}
	c.handshake = &handshake
	return nil
}

func (c *rpcPluginClient) requestedHooks() []string {
	hooks := []string{}
	if c.meta.Permissions.Render.Context.Read || c.meta.Permissions.Render.Context.Write {
		hooks = append(hooks, pluginrpc.MethodContext)
	}
	if c.meta.Permissions.Render.AfterRender.MutateHTML {
		hooks = append(hooks, pluginrpc.MethodAfterRender)
	}
	if c.meta.Permissions.Render.HTMLSlots.Inject {
		hooks = append(hooks, pluginrpc.MethodHTMLSlots)
	}
	return hooks
}

func (c *rpcPluginClient) callLocked(method string, params any, out any) error {
	// Kill closes the pipe and releases a blocked encoder/decoder. Keeping the
	// deadline active through both write and read also bounds a peer that stops
	// consuming input. Wait is performed after the operation unwinds.
	process := c.cmd.Process
	input, output := c.input, c.output
	deadlineDone := make(chan struct{})
	timer := time.AfterFunc(10*time.Second, func() {
		defer close(deadlineDone)
		killRPCProcess(process)
		_ = input.Close()
		_ = output.Close()
	})
	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			if !timer.Stop() {
				<-deadlineDone
			}
		})
	}
	c.cancelDeadline = cancel
	defer func() { cancel(); c.cancelDeadline = nil }()
	c.nextID++
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if len(body) > 8*1024*1024 {
		return fmt.Errorf("RPC request exceeds 8 MiB limit")
	}
	if err := c.encoder.Encode(pluginrpc.Request{
		ID:     c.nextID,
		Method: method,
		Params: body,
	}); err != nil {
		c.stopLocked()
		return err
	}
	if err := c.stdin.Flush(); err != nil {
		c.stopLocked()
		return err
	}
	var resp pluginrpc.Response
	if !c.scanner.Scan() {
		err := c.scanner.Err()
		if err == nil {
			err = fmt.Errorf("RPC peer closed its output")
		}
		c.stopLocked()
		return err
	}
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		c.stopLocked()
		return err
	}
	if resp.ID != c.nextID {
		c.stopLocked()
		return fmt.Errorf("rpc plugin %q returned an unexpected response ID", c.meta.Name)
	}
	if resp.Error != "" {
		return fmt.Errorf("rpc plugin %q %s failed: %s", c.meta.Name, method, resp.Error)
	}
	if out != nil {
		if len(resp.Result) == 0 || string(resp.Result) == "null" {
			c.stopLocked()
			return fmt.Errorf("rpc plugin %q omitted its result", c.meta.Name)
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			c.stopLocked()
			return err
		}
	}
	cancel()
	select {
	case <-deadlineDone:
		c.stopLocked()
		return fmt.Errorf("rpc plugin %q %s exceeded its 10-second deadline", c.meta.Name, method)
	default:
	}
	return nil
}

func (c *rpcPluginClient) stopLocked() {
	// Finish any racing deadline callback before reaping the process, preventing
	// a late process-group signal from targeting a recycled PID.
	if c.cancelDeadline != nil {
		c.cancelDeadline()
	}
	if c.cmd != nil {
		killRPCProcess(c.cmd.Process)
		_ = c.input.Close()
		_ = c.output.Close()
		_ = c.cmd.Wait()
	}
	c.cmd = nil
	c.input = nil
	c.output = nil
	c.handshake = nil
	c.stdin = nil
	c.encoder = nil
	c.scanner = nil
}

// Close releases RPC processes owned by this manager. Callers can close even
// when no hook was ever invoked; no plugin is started just to shut it down.
func (m *Manager) Close() error {
	for _, plugin := range m.plugins {
		if proxy, ok := plugin.(*rpcPluginProxy); ok {
			proxy.client.mu.Lock()
			proxy.client.stopLocked()
			proxy.client.mu.Unlock()
		}
	}
	return nil
}

func (c *rpcPluginClient) rpcEnv() []string {
	path := ""
	home := ""
	goCache := ""
	tmpDir := ""
	for _, item := range os.Environ() {
		if c.meta.Runtime.Sandbox.Profile == "strict" {
			break
		}
		if strings.HasPrefix(item, "PATH=") {
			path = item
		}
		if strings.HasPrefix(item, "HOME=") {
			home = item
		}
		if strings.HasPrefix(item, "GOCACHE=") {
			goCache = item
		}
		if strings.HasPrefix(item, "TMPDIR=") {
			tmpDir = item
		}
	}
	env := []string{}
	if path != "" {
		env = append(env, path)
	}
	if home != "" {
		env = append(env, home)
	}
	if goCache != "" {
		env = append(env, goCache)
	}
	if tmpDir != "" {
		env = append(env, tmpDir)
	}
	env = append(env,
		"FOUNDRY_PLUGIN_NAME="+c.meta.Name,
		"FOUNDRY_PLUGIN_DIR="+c.meta.Directory,
		"FOUNDRY_PLUGIN_PROTOCOL="+c.meta.Runtime.ProtocolVersion,
	)
	for key, value := range c.meta.Runtime.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func hookSupported(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

func toRPCContextRequest(view *renderer.ViewData) pluginrpc.ContextRequest {
	req := pluginrpc.ContextRequest{
		Data:        map[string]any{},
		Lang:        view.Lang,
		Title:       view.Title,
		RequestPath: view.RequestPath,
	}
	for key, value := range view.Data {
		req.Data[key] = value
	}
	if view.Page != nil {
		req.Page = &pluginrpc.PagePayload{
			ID:         view.Page.ID,
			Type:       view.Page.Type,
			Lang:       view.Page.Lang,
			Status:     view.Page.Status,
			Title:      view.Page.Title,
			Slug:       view.Page.Slug,
			URL:        view.Page.URL,
			Layout:     view.Page.Layout,
			Summary:    view.Page.Summary,
			Draft:      view.Page.Draft,
			RawBody:    view.Page.RawBody,
			HTMLBody:   string(view.Page.HTMLBody),
			Params:     cloneMap(view.Page.Params),
			Fields:     cloneMap(view.Page.Fields),
			Taxonomies: cloneTaxonomies(view.Page.Taxonomies),
		}
	}
	return req
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneTaxonomies(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

var (
	_ Plugin          = (*rpcPluginProxy)(nil)
	_ ContextHook     = (*rpcPluginProxy)(nil)
	_ AfterRenderHook = (*rpcPluginProxy)(nil)
	_ HTMLSlotsHook   = (*rpcPluginProxy)(nil)
)
