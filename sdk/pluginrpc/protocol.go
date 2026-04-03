package pluginrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const (
	MethodHandshake = "handshake"
	MethodContext   = "context"
	MethodShutdown  = "shutdown"
)

type Request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type HandshakeRequest struct {
	PluginName       string   `json:"plugin_name"`
	ProtocolVersion  string   `json:"protocol_version"`
	RequestedHooks   []string `json:"requested_hooks,omitempty"`
	SandboxProfile   string   `json:"sandbox_profile,omitempty"`
	AllowNetwork     bool     `json:"allow_network,omitempty"`
	AllowFSWrite     bool     `json:"allow_filesystem_write,omitempty"`
	AllowProcessExec bool     `json:"allow_process_exec,omitempty"`
}

type HandshakeResponse struct {
	PluginName      string   `json:"plugin_name"`
	ProtocolVersion string   `json:"protocol_version"`
	SupportedHooks  []string `json:"supported_hooks,omitempty"`
}

type ContextRequest struct {
	Page        *PagePayload   `json:"page,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Lang        string         `json:"lang,omitempty"`
	Title       string         `json:"title,omitempty"`
	RequestPath string         `json:"request_path,omitempty"`
}

type ContextResponse struct {
	Data map[string]any `json:"data,omitempty"`
}

type PagePayload struct {
	ID         string              `json:"id,omitempty"`
	Type       string              `json:"type,omitempty"`
	Lang       string              `json:"lang,omitempty"`
	Status     string              `json:"status,omitempty"`
	Title      string              `json:"title,omitempty"`
	Slug       string              `json:"slug,omitempty"`
	URL        string              `json:"url,omitempty"`
	Layout     string              `json:"layout,omitempty"`
	Summary    string              `json:"summary,omitempty"`
	Draft      bool                `json:"draft,omitempty"`
	Archived   bool                `json:"archived,omitempty"`
	RawBody    string              `json:"raw_body,omitempty"`
	HTMLBody   string              `json:"html_body,omitempty"`
	Params     map[string]any      `json:"params,omitempty"`
	Fields     map[string]any      `json:"fields,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type Handler interface {
	Handshake(HandshakeRequest) (HandshakeResponse, error)
	Context(ContextRequest) (ContextResponse, error)
	Shutdown() error
}

type Server struct {
	Reader io.Reader
	Writer io.Writer
}

func (s Server) Serve(handler Handler) error {
	decoder := json.NewDecoder(bufio.NewReader(s.Reader))
	encoder := json.NewEncoder(s.Writer)
	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		resp := Response{ID: req.ID}
		switch req.Method {
		case MethodHandshake:
			var body HandshakeRequest
			if err := json.Unmarshal(req.Params, &body); err != nil {
				resp.Error = err.Error()
			} else if result, err := handler.Handshake(body); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = mustJSON(result)
			}
		case MethodContext:
			var body ContextRequest
			if err := json.Unmarshal(req.Params, &body); err != nil {
				resp.Error = err.Error()
			} else if result, err := handler.Context(body); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = mustJSON(result)
			}
		case MethodShutdown:
			if err := handler.Shutdown(); err != nil {
				resp.Error = err.Error()
			}
			if err := encoder.Encode(resp); err != nil {
				return err
			}
			return nil
		default:
			resp.Error = fmt.Sprintf("unsupported method %q", req.Method)
		}
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
}

func mustJSON(v any) json.RawMessage {
	body, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return body
}
