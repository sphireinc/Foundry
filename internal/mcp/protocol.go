package mcp

import "encoding/json"

const ProtocolVersion = "2026-07-28"

var supportedProtocolVersions = []string{"2026-07-28", "2025-06-18"}

func SupportedProtocolVersions() []string {
	out := make([]string, len(supportedProtocolVersions))
	copy(out, supportedProtocolVersions)
	return out
}

const (
	jsonRPCVersion     = "2.0"
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeServerBusy     = -32001
	codeRateLimited    = -32002
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (r rpcRequest) isNotification() bool {
	return len(r.ID) == 0
}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
