package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/sphireinc/foundry/internal/logx"
)

const maxStdioLineBytes = 8 * 1024 * 1024

func (s *Server) ServeStdio(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewScanner(stdin)
	reader.Buffer(make([]byte, 0, 64*1024), maxStdioLineBytes)
	writer := json.NewEncoder(stdout)

	for reader.Scan() {
		line := bytes.TrimSpace(reader.Bytes())
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			if err := encodeResponse(writer, rpcResponse{
				JSONRPC: jsonRPCVersion,
				Error:   &rpcError{Code: codeParseError, Message: "parse error"},
			}); err != nil {
				return err
			}
			continue
		}

		if req.Method == "" {
			if err := encodeResponse(writer, rpcResponse{
				JSONRPC: jsonRPCVersion,
				ID:      req.ID,
				Error:   &rpcError{Code: codeInvalidRequest, Message: "method is required"},
			}); err != nil {
				return err
			}
			continue
		}

		if req.isNotification() {
			s.dispatch(ctx, req)
			continue
		}

		result, rpcErr, _ := s.handle(ctx, req)
		resp := rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		if err := encodeResponse(writer, resp); err != nil {
			return err
		}
	}

	if err := reader.Err(); err != nil {
		logx.Error("mcp stdio reader stopped", "error", err)
		return err
	}
	logx.Info("mcp stdio input closed")
	return nil
}

func encodeResponse(writer *json.Encoder, resp rpcResponse) error {
	if writer == nil {
		return nil
	}
	return writer.Encode(resp)
}
