package httpadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/service"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
)

func TestCapabilitiesEndpoint(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/capabilities", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var caps admintypes.CapabilityResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &caps); err != nil {
		t.Fatalf("decode capability response: %v", err)
	}
	if !caps.Modules["documents"] || !caps.Features["structured_editing"] {
		t.Fatalf("unexpected capability response: %#v", caps)
	}
}

func TestSettingsSectionsAndExtensionsEndpoints(t *testing.T) {
	cfg := testConfig(t)
	svc := service.New(cfg,
		service.WithGraphLoader(func(_ context.Context, _ *config.Config, _ bool) (*content.SiteGraph, error) {
			return content.NewSiteGraph(cfg), nil
		}),
		service.WithPluginMetadata(func() map[string]plugins.Metadata {
			return map[string]plugins.Metadata{
				"search": {
					Name: "search",
					AdminExtensions: plugins.AdminExtensions{
						Pages:            []plugins.AdminPage{{Key: "search-console", Title: "Search Console", Route: "/plugins/search", Module: "admin/search-console.js", Styles: []string{"admin/search-console.css"}}},
						Widgets:          []plugins.AdminWidget{{Key: "search-status", Title: "Search Status", Slot: "overview.after", Module: "admin/search-widget.js", Styles: []string{"admin/search-widget.css"}}},
						SettingsSections: []plugins.AdminSettingsSection{{Key: "search", Title: "Search", Description: "Search tuning"}},
					},
				},
			}
		}),
	)
	r := New(cfg, svc)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	doReq := func(path string, target any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:10000"
		req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, rr.Code, rr.Body.String())
		}
		if err := json.Unmarshal(rr.Body.Bytes(), target); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}

	var sections []admintypes.SettingsSection
	doReq(cfg.AdminPath()+"/api/settings/sections", &sections)
	foundPluginSection := false
	for _, section := range sections {
		if section.Source == "search" && section.Key == "search" {
			foundPluginSection = true
			break
		}
	}
	if !foundPluginSection {
		t.Fatalf("expected plugin-defined settings section, got %#v", sections)
	}

	var registry admintypes.AdminExtensionRegistry
	doReq(cfg.AdminPath()+"/api/extensions", &registry)
	if len(registry.Pages) != 1 || registry.Pages[0].Plugin != "search" {
		t.Fatalf("expected plugin extension page, got %#v", registry)
	}
	if registry.Pages[0].ModuleURL != cfg.AdminPath()+"/extensions/search/admin/search-console.js" {
		t.Fatalf("expected normalized module url, got %#v", registry.Pages[0])
	}
	if len(registry.Pages[0].StyleURLs) != 1 || registry.Pages[0].StyleURLs[0] != cfg.AdminPath()+"/extensions/search/admin/search-console.css" {
		t.Fatalf("expected normalized style urls, got %#v", registry.Pages[0])
	}
	if len(registry.Widgets) != 1 || registry.Widgets[0].ModuleURL != cfg.AdminPath()+"/extensions/search/admin/search-widget.js" {
		t.Fatalf("expected normalized widget module url, got %#v", registry.Widgets)
	}
}
