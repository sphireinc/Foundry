package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/feed"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/site"
)

type TimingBreakdown struct {
	PluginConfig time.Duration
	Loader       time.Duration
	Router       time.Duration
	RouteHooks   time.Duration
	Assets       time.Duration
	Renderer     time.Duration
	Feed         time.Duration
}

type PreviewLink struct {
	Title      string `json:"title"`
	Status     string `json:"status"`
	Type       string `json:"type"`
	Lang       string `json:"lang"`
	SourcePath string `json:"source_path"`
	URL        string `json:"url"`
	PreviewURL string `json:"preview_url"`
}

type PreviewManifest struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Environment string        `json:"environment"`
	Target      string        `json:"target,omitempty"`
	Links       []PreviewLink `json:"links"`
}

type BuildReport struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	Environment   string              `json:"environment"`
	Target        string              `json:"target,omitempty"`
	Preview       bool                `json:"preview"`
	DocumentCount int                 `json:"document_count"`
	RouteCount    int                 `json:"route_count"`
	Stats         renderer.BuildStats `json:"stats"`
}

type TimedRouteHooks struct {
	Hooks    site.RouteHooks
	Duration time.Duration
}

func (h *TimedRouteHooks) OnRoutesAssigned(graph *content.SiteGraph) error {
	if h == nil || h.Hooks == nil {
		return nil
	}
	start := time.Now()
	err := h.Hooks.OnRoutesAssigned(graph)
	h.Duration += time.Since(start)
	return err
}

func LoadGraphWithTiming(ctx context.Context, loader site.Loader, resolver *router.Resolver, hooks site.RouteHooks) (*content.SiteGraph, TimingBreakdown, error) {
	var metrics TimingBreakdown
	start := time.Now()
	graph, err := loader.Load(ctx)
	metrics.Loader = time.Since(start)
	if err != nil {
		return nil, metrics, err
	}

	start = time.Now()
	if err := resolver.AssignURLs(graph); err != nil {
		return nil, metrics, err
	}
	metrics.Router = time.Since(start)

	if hooks != nil {
		start = time.Now()
		if err := hooks.OnRoutesAssigned(graph); err != nil {
			return nil, metrics, err
		}
		metrics.RouteHooks = time.Since(start)
	}

	return graph, metrics, nil
}

func BuildFeedsWithTiming(cfg *config.Config, graph *content.SiteGraph) (TimingBreakdown, error) {
	var metrics TimingBreakdown
	start := time.Now()
	if _, _, err := feed.Build(cfg, graph); err != nil {
		return metrics, err
	}
	metrics.Feed = time.Since(start)
	return metrics, nil
}

func BuildRendererWithTiming(ctx context.Context, r *renderer.Renderer, graph *content.SiteGraph) (TimingBreakdown, error) {
	var metrics TimingBreakdown
	start := time.Now()
	stats, err := r.BuildWithStats(ctx, graph)
	if err != nil {
		return metrics, err
	}
	metrics.Renderer = time.Since(start)
	metrics.Assets = stats.Assets
	return metrics, nil
}

func WritePreviewManifest(cfg *config.Config, graph *content.SiteGraph, target string, enabled bool) error {
	manifestPath := filepath.Join(cfg.PublicDir, "preview-links.json")
	if !enabled {
		if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove preview manifest: %w", err)
		}
		return nil
	}

	links := make([]PreviewLink, 0)
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		if doc.Status == "published" && !doc.Draft {
			continue
		}
		previewURL := doc.URL
		if baseURL != "" {
			previewURL = baseURL + doc.URL
		}
		links = append(links, PreviewLink{
			Title:      doc.Title,
			Status:     doc.Status,
			Type:       doc.Type,
			Lang:       doc.Lang,
			SourcePath: doc.SourcePath,
			URL:        doc.URL,
			PreviewURL: previewURL,
		})
	}

	sort.Slice(links, func(i, j int) bool {
		if links[i].Status != links[j].Status {
			return links[i].Status < links[j].Status
		}
		if links[i].Lang != links[j].Lang {
			return links[i].Lang < links[j].Lang
		}
		return links[i].URL < links[j].URL
	})

	if err := os.MkdirAll(cfg.PublicDir, 0o755); err != nil {
		return fmt.Errorf("mkdir public dir: %w", err)
	}
	body, err := json.MarshalIndent(PreviewManifest{
		GeneratedAt: time.Now().UTC(),
		Environment: cfg.Environment,
		Target:      target,
		Links:       links,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preview manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write preview manifest: %w", err)
	}
	return nil
}

func WriteBuildReport(cfg *config.Config, graph *content.SiteGraph, target string, preview bool, stats renderer.BuildStats) error {
	if cfg == nil {
		return nil
	}
	path := filepath.Join(cfg.DataDir, "admin", "build-report.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir build report dir: %w", err)
	}
	report := BuildReport{
		GeneratedAt:   time.Now().UTC(),
		Environment:   cfg.Environment,
		Target:        strings.TrimSpace(target),
		Preview:       preview,
		DocumentCount: 0,
		RouteCount:    0,
		Stats:         stats,
	}
	if graph != nil {
		report.DocumentCount = len(graph.Documents)
		report.RouteCount = len(graph.ByURL)
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal build report: %w", err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write build report: %w", err)
	}
	return nil
}
