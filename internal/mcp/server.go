package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ServerOptions struct {
	Registry   *Registry
	Authorizer *Authorizer
	Auditor    Auditor
	Actor      Actor
	Info       Implementation
	Now        func() time.Time
}

type Server struct {
	registry   *Registry
	authorizer *Authorizer
	auditor    Auditor
	actor      Actor
	info       Implementation
	limiter    *rateLimiter
	slots      chan struct{}
	clock      func() time.Time
}

func NewServer(opts ServerOptions) (*Server, error) {
	if opts.Registry == nil {
		return nil, fmt.Errorf("mcp: registry is required")
	}

	authorizer := opts.Authorizer
	if authorizer == nil {
		authorizer = NewAuthorizer(DefaultPolicy())
	}

	auditor := opts.Auditor
	if auditor == nil {
		auditor = noopAuditor{}
	}

	info := opts.Info
	if strings.TrimSpace(info.Name) == "" {
		info.Name = "foundry"
	}

	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}

	policy := authorizer.Policy()
	limits := RateLimit{}
	maxConcurrency := 0
	if policy != nil {
		limits = policy.RateLimit()
		maxConcurrency = policy.MaxConcurrency()
	}

	var slots chan struct{}
	if maxConcurrency > 0 {
		slots = make(chan struct{}, maxConcurrency)
	}

	return &Server{
		registry:   opts.Registry,
		authorizer: authorizer,
		auditor:    auditor,
		actor:      opts.Actor,
		info:       info,
		limiter:    newRateLimiter(limits.RequestsPerMinute, limits.Burst, clock()),
		slots:      slots,
		clock:      clock,
	}, nil
}

func (s *Server) Registry() *Registry {
	return s.registry
}

func (s *Server) handle(ctx context.Context, req rpcRequest) (any, *rpcError, bool) {
	if !s.acquire() {
		return nil, &rpcError{Code: codeServerBusy, Message: "concurrency limit exceeded; retry later"}, false
	}
	defer s.release()

	if !s.limiter.allow(s.clock()) {
		return nil, &rpcError{Code: codeRateLimited, Message: "rate limit exceeded; slow down"}, false
	}

	return s.dispatch(ctx, req)
}

func (s *Server) acquire() bool {
	if s.slots == nil {
		return true
	}
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) release() {
	if s.slots != nil {
		<-s.slots
	}
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError, bool) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil, false
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil, true
	case "ping":
		return map[string]any{}, nil, false
	case "tools/list":
		return s.listTools(), nil, false
	case "tools/call":
		result, err := s.callTool(ctx, req.Params)
		return result, err, false
	case "resources/list":
		return s.listResources(), nil, false
	case "resources/templates/list":
		return s.listResourceTemplates(), nil, false
	case "resources/read":
		result, err := s.readResource(ctx, req.Params)
		return result, err, false
	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil, false
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}, false
	}
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ClientInfo      Implementation `json:"clientInfo"`
}

func (s *Server) initialize(params json.RawMessage) any {
	var input initializeParams
	_ = json.Unmarshal(params, &input)

	version := ProtocolVersion
	if input.ProtocolVersion != "" {
		for _, supported := range supportedProtocolVersions {
			if input.ProtocolVersion == supported {
				version = supported
				break
			}
		}
	}

	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{"subscribe": false, "listChanged": false},
		},
		"serverInfo": s.info,
	}
}

type toolDescriptor struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	InputSchema *Schema `json:"inputSchema"`
}

func (s *Server) listTools() any {
	tools := make([]toolDescriptor, 0)
	for _, cap := range s.registry.Tools() {
		if !s.authorizer.Policy().ScopeEnabled(cap.Scope) {
			continue
		}
		schema := cap.InputSchema
		if schema == nil {
			schema = ObjectSchema(map[string]*Schema{})
		}
		tools = append(tools, toolDescriptor{
			Name:        cap.Name,
			Description: cap.Description,
			InputSchema: schema,
		})
	}
	return map[string]any{"tools": tools}
}

type resourceDescriptor struct {
	URI         string `json:"uri,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	URITemplate string `json:"uriTemplate,omitempty"`
}

func (s *Server) listResources() any {
	resources := make([]resourceDescriptor, 0)
	for _, cap := range s.registry.Resources() {
		if !s.authorizer.Policy().ScopeEnabled(cap.Scope) {
			continue
		}
		if strings.Contains(cap.URITemplate, "{") {
			continue
		}
		resources = append(resources, resourceDescriptor{
			URI:         cap.URITemplate,
			Name:        cap.Name,
			Description: cap.Description,
			MimeType:    "application/json",
		})
	}
	return map[string]any{"resources": resources}
}

func (s *Server) listResourceTemplates() any {
	templates := make([]resourceDescriptor, 0)
	for _, cap := range s.registry.Resources() {
		if !s.authorizer.Policy().ScopeEnabled(cap.Scope) {
			continue
		}
		if !strings.Contains(cap.URITemplate, "{") {
			continue
		}
		templates = append(templates, resourceDescriptor{
			Name:        cap.Name,
			Description: cap.Description,
			URITemplate: cap.URITemplate,
			MimeType:    "application/json",
		})
	}
	return map[string]any{"resourceTemplates": templates}
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var input callToolParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid tool call parameters"}
	}

	cap, ok := s.registry.Tool(input.Name)
	if !ok {
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool: " + input.Name}
	}
	if err := s.authorizer.Authorize(cap, s.actor); err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.Name, "denied", map[string]string{"error": err.Error()}))
		return toolErrorContent(err), nil
	}

	callCtx, cancel := context.WithTimeout(ctx, cap.Timeout)
	defer cancel()

	_ = s.auditor.Log(AuditEntry(s.actor, cap, input.Name, "started", nil))

	result, err := cap.Handler(callCtx, Invocation{
		Capability: cap.Name,
		Input:      input.Arguments,
	})
	if err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.Name, "error", map[string]string{"error": err.Error()}))
		return toolErrorContent(err), nil
	}

	data, err := MarshalBounded(RedactJSON(result), cap.MaxResultBytes)
	if err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.Name, "error", map[string]string{"error": err.Error()}))
		return toolErrorContent(err), nil
	}

	_ = s.auditor.Log(AuditEntry(s.actor, cap, input.Name, "success", nil))
	return map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": string(data)},
		},
		"isError": false,
	}, nil
}

type readResourceParams struct {
	URI string `json:"uri"`
}

func (s *Server) readResource(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var input readResourceParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid resource read parameters"}
	}
	if strings.TrimSpace(input.URI) == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "resource uri is required"}
	}

	cap, vars, ok := s.registry.LookupURI(input.URI)
	if !ok {
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown resource: " + input.URI}
	}
	if err := s.authorizer.Authorize(cap, s.actor); err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.URI, "denied", map[string]string{"error": err.Error()}))
		return nil, &rpcError{Code: codeInvalidRequest, Message: err.Error()}
	}

	callCtx, cancel := context.WithTimeout(ctx, cap.Timeout)
	defer cancel()

	_ = s.auditor.Log(AuditEntry(s.actor, cap, input.URI, "started", nil))

	result, err := cap.Handler(callCtx, Invocation{
		Capability: cap.Name,
		URI:        input.URI,
		Vars:       vars,
	})
	if err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.URI, "error", map[string]string{"error": err.Error()}))
		return nil, &rpcError{Code: codeInternalError, Message: err.Error()}
	}

	data, err := MarshalBounded(RedactJSON(result), cap.MaxResultBytes)
	if err != nil {
		_ = s.auditor.Log(AuditEntry(s.actor, cap, input.URI, "error", map[string]string{"error": err.Error()}))
		return nil, &rpcError{Code: codeInternalError, Message: err.Error()}
	}

	_ = s.auditor.Log(AuditEntry(s.actor, cap, input.URI, "success", nil))
	return map[string]any{
		"contents": []any{
			map[string]any{"uri": input.URI, "mimeType": "application/json", "text": string(data)},
		},
	}, nil
}

func toolErrorContent(err error) any {
	return map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": err.Error()},
		},
		"isError": true,
	}
}
