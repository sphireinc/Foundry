package plugins

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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
	resp, err := p.client.Context(toRPCContextRequest(ctx))
	if err != nil {
		return err
	}
	if len(resp.Data) == 0 {
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

type rpcPluginClient struct {
	meta      Metadata
	mu        sync.Mutex
	cmd       *exec.Cmd
	stdin     *bufio.Writer
	encoder   *json.Encoder
	decoder   *json.Decoder
	nextID    int
	handshake *pluginrpc.HandshakeResponse
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
	cmd := exec.Command(c.meta.Runtime.Command[0], c.meta.Runtime.Command[1:]...)
	cmd.Dir = c.meta.Directory
	cmd.Env = c.rpcEnv()
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.stdin = bufio.NewWriter(stdinPipe)
	c.encoder = json.NewEncoder(c.stdin)
	c.decoder = json.NewDecoder(bufio.NewReader(stdoutPipe))

	var handshake pluginrpc.HandshakeResponse
	if err := c.callLocked(pluginrpc.MethodHandshake, pluginrpc.HandshakeRequest{
		PluginName:       c.meta.Name,
		ProtocolVersion:  c.meta.Runtime.ProtocolVersion,
		RequestedHooks:   []string{pluginrpc.MethodContext},
		SandboxProfile:   c.meta.Runtime.Sandbox.Profile,
		AllowNetwork:     c.meta.Runtime.Sandbox.AllowNetwork,
		AllowFSWrite:     c.meta.Runtime.Sandbox.AllowFilesystemWrite,
		AllowProcessExec: c.meta.Runtime.Sandbox.AllowProcessExec,
	}, &handshake); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	c.handshake = &handshake
	return nil
}

func (c *rpcPluginClient) callLocked(method string, params any, out any) error {
	c.nextID++
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := c.encoder.Encode(pluginrpc.Request{
		ID:     c.nextID,
		Method: method,
		Params: body,
	}); err != nil {
		return err
	}
	if err := c.stdin.Flush(); err != nil {
		return err
	}
	var resp pluginrpc.Response
	if err := c.decoder.Decode(&resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("rpc plugin %q %s failed: %s", c.meta.Name, method, resp.Error)
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return err
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
	_ Plugin      = (*rpcPluginProxy)(nil)
	_ ContextHook = (*rpcPluginProxy)(nil)
)
