package server

import (
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/theme"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedMediaRouteServesStaticVariants(t *testing.T) {
	cfg := testServerConfig(t)
	directory := filepath.Join(cfg.PublicDir, "_foundry", "media")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "variant.png"), []byte("variant"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := New(cfg, stubLoader{}, router.NewResolver(cfg), renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), &hookRecorder{}, false)
	response := httptest.NewRecorder()
	server.newMux().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_foundry/media/variant.png", nil))
	if response.Code != http.StatusOK || response.Body.String() != "variant" {
		t.Fatalf("variant request failed: %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("media security headers missing")
	}
}
