package main

import (
	"fmt"
	"github.com/sphireinc/foundry/sdk/pluginrpc"
	"os"
	"strings"
)

type handler struct{}

func (handler) Handshake(req pluginrpc.HandshakeRequest) (pluginrpc.HandshakeResponse, error) {
	return pluginrpc.HandshakeResponse{PluginName: req.PluginName, ProtocolVersion: req.ProtocolVersion, SupportedHooks: []string{pluginrpc.MethodContext}}, nil
}
func (handler) Context(req pluginrpc.ContextRequest) (pluginrpc.ContextResponse, error) {
	words := 0
	if req.Page != nil {
		words = len(strings.Fields(req.Page.RawBody))
	}
	return pluginrpc.ContextResponse{Data: map[string]any{"wordcount_rpc": map[string]any{"words": words}}}, nil
}
func (handler) Shutdown() error { return nil }
func main() {
	server := pluginrpc.Server{Reader: os.Stdin, Writer: os.Stdout}
	if err := server.Serve(handler{}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
