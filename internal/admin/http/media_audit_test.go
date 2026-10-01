package httpadmin

import (
	"encoding/json"
	"github.com/sphireinc/foundry/internal/media"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaAuditEndpointIsReadOnlyAndAuthenticated(t *testing.T) {
	cfg := testConfig(t)
	router := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	router.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/__admin/api/media/audit", nil)
	req.RemoteAddr = "8.8.8.8:10000"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated audit returned %d", response.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/__admin/api/media/audit", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("audit returned %d: %s", response.Code, response.Body.String())
	}
	var report media.AuditReport
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Notes) == 0 {
		t.Fatal("orphan caveat absent from report")
	}
	req = httptest.NewRequest(http.MethodPost, "/__admin/api/media/audit", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("audit accepted POST: %d", response.Code)
	}
}
