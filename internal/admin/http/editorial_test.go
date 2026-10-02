package httpadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEditorialEndpointRejectsUnauthenticatedAndBypassedPublication(t *testing.T) {
	cfg := testConfig(t)
	cfg.Editorial.RequireApproval = true
	router := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	router.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/editorial", strings.NewReader(`{"action":"approve"}`))
	request.RemoteAddr = "8.8.8.8:12345"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated action returned %d", response.Code)
	}
	for _, test := range []struct{ path, body string }{
		{"/api/documents/status", `{"source_path":"pages/about.md","status":"published"}`},
		{"/api/documents/editorial", `{"source_path":"pages/about.md","action":"approve","expected_revision":"stale"}`},
	} {
		request := httptest.NewRequest(http.MethodPost, "/__admin"+test.path, strings.NewReader(test.body))
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("bypass returned %d: %s", response.Code, response.Body.String())
		}
	}
}
