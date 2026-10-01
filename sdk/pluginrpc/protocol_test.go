package pluginrpc

import (
	"bytes"
	"encoding/json"
	"testing"
)

type legacyHandler struct{}

func (legacyHandler) Handshake(req HandshakeRequest) (HandshakeResponse, error) {
	return HandshakeResponse{PluginName: req.PluginName, ProtocolVersion: req.ProtocolVersion, SupportedHooks: []string{MethodContext}}, nil
}
func (legacyHandler) Context(ContextRequest) (ContextResponse, error) {
	return ContextResponse{Data: map[string]any{"legacy": true}}, nil
}
func (legacyHandler) Shutdown() error { return nil }

type renderingHandler struct{ legacyHandler }

func (renderingHandler) HTMLSlots(ContextRequest) (HTMLSlotsResponse, error) {
	return HTMLSlotsResponse{Slots: map[string][]string{"head": {"<meta name=test>"}}}, nil
}
func (renderingHandler) AfterRender(req AfterRenderRequest) (AfterRenderResponse, error) {
	return AfterRenderResponse{HTML: req.HTML + "!"}, nil
}

func TestServerOptionalRenderingHooks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		handler   Handler
		supported bool
	}{
		{"legacy", legacyHandler{}, false},
		{"rendering", renderingHandler{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input, output bytes.Buffer
			enc := json.NewEncoder(&input)
			for i, method := range []string{MethodHTMLSlots, MethodAfterRender, MethodContext, MethodShutdown} {
				params := json.RawMessage(`{"html":"original"}`)
				if err := enc.Encode(Request{ID: i + 1, Method: method, Params: params}); err != nil {
					t.Fatal(err)
				}
			}
			if err := (Server{Reader: &input, Writer: &output}).Serve(tc.handler); err != nil {
				t.Fatal(err)
			}
			dec := json.NewDecoder(&output)
			for i := 1; i <= 4; i++ {
				var resp Response
				if err := dec.Decode(&resp); err != nil {
					t.Fatal(err)
				}
				if resp.ID != i {
					t.Fatalf("incorrect response correlation: %#v", resp)
				}
				if i <= 2 && !tc.supported {
					if resp.Error == "" {
						t.Fatal("legacy handler silently accepted unsupported rendering method")
					}
					continue
				}
				if resp.Error != "" {
					t.Fatal(resp.Error)
				}
				if i == 2 {
					var body AfterRenderResponse
					if err := json.Unmarshal(resp.Result, &body); err != nil {
						t.Fatal(err)
					}
					if body.HTML != "original!" {
						t.Fatal("after-render output not returned")
					}
				}
			}
		})
	}
}
