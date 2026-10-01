package main

import (
	"os"

	"github.com/sphireinc/foundry/sdk/pluginrpc"
)

type handler struct{}

func (handler) Handshake(req pluginrpc.HandshakeRequest) (pluginrpc.HandshakeResponse, error) {
	return pluginrpc.HandshakeResponse{
		PluginName:      req.PluginName,
		ProtocolVersion: req.ProtocolVersion,
		SupportedHooks:  []string{pluginrpc.MethodContext, pluginrpc.MethodHTMLSlots, pluginrpc.MethodAfterRender},
	}, nil
}

func (handler) Context(req pluginrpc.ContextRequest) (pluginrpc.ContextResponse, error) {
	out := map[string]any{
		"rpc_context_demo": map[string]any{
			"enabled": true,
			"title":   req.Title,
			"page_id": func() string {
				if req.Page != nil {
					return req.Page.ID
				}
				return ""
			}(),
		},
	}
	return pluginrpc.ContextResponse{Data: out}, nil
}

func (handler) Shutdown() error { return nil }

func (handler) HTMLSlots(pluginrpc.ContextRequest) (pluginrpc.HTMLSlotsResponse, error) {
	return pluginrpc.HTMLSlotsResponse{Slots: map[string][]string{"head": {"<!-- RPC slot demo -->"}}}, nil
}

func (handler) AfterRender(req pluginrpc.AfterRenderRequest) (pluginrpc.AfterRenderResponse, error) {
	return pluginrpc.AfterRenderResponse{HTML: req.HTML + "\n<!-- RPC after-render demo -->"}, nil
}

func main() {
	server := pluginrpc.Server{
		Reader: os.Stdin,
		Writer: os.Stdout,
	}
	if err := server.Serve(handler{}); err != nil {
		os.Exit(1)
	}
}
