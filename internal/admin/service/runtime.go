package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/audit"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/media"
	"github.com/sphireinc/foundry/internal/ops"
	"gopkg.in/yaml.v3"
)

var serviceProcessStartedAt = time.Now().UTC()

const runtimeAuditWindow = 24 * time.Hour

type runtimeSessionFile struct {
	Sessions []struct {
		Token     string    `yaml:"token"`
		ExpiresAt time.Time `yaml:"expires_at"`
	} `yaml:"sessions"`
}

func (s *Service) GetRuntimeStatus(ctx context.Context) (*types.RuntimeStatus, error) {
	if err := requireCapability(ctx, "debug.read"); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	userCPU, systemCPU := processCPUTime()

	status := &types.RuntimeStatus{
		CapturedAt:         now,
		UptimeSeconds:      int64(time.Since(serviceProcessStartedAt).Seconds()),
		GoVersion:          runtime.Version(),
		NumCPU:             runtime.NumCPU(),
		LiveReloadMode:     s.cfg.Server.LiveReloadMode,
		HeapAllocBytes:     mem.HeapAlloc,
		HeapInuseBytes:     mem.HeapInuse,
		HeapObjects:        mem.HeapObjects,
		StackInuseBytes:    mem.StackInuse,
		SysBytes:           mem.Sys,
		NumGC:              mem.NumGC,
		NextGCBytes:        mem.NextGC,
		Goroutines:         runtime.NumGoroutine(),
		ProcessUserCPUMS:   userCPU.Milliseconds(),
		ProcessSystemCPUMS: systemCPU.Milliseconds(),
		Content: types.RuntimeContentStatus{
			ByType:      map[string]int{},
			ByLang:      map[string]int{},
			ByStatus:    map[string]int{},
			MediaCounts: map[string]int{},
		},
		Storage: types.RuntimeStorageStatus{
			MediaBytes:   map[string]int64{},
			MediaCounts:  map[string]int{},
			LargestFiles: make([]types.RuntimeFileStat, 0),
		},
		Integrity: types.RuntimeIntegrityStatus{},
		Activity: types.RuntimeActivityStatus{
			RecentAuditByAction: map[string]int{},
			AuditWindowHours:    int(runtimeAuditWindow / time.Hour),
		},
	}
	if mem.LastGC > 0 {
		lastGC := time.Unix(0, int64(mem.LastGC)).UTC()
		status.LastGCAt = &lastGC
	}

	s.populateRuntimeGraphMetrics(ctx, status)
	s.populateRuntimeStorageMetrics(status)
	s.populateRuntimeActivityMetrics(status, now)
	s.populateRuntimeBuildStatus(status)

	return status, nil
}

func (s *Service) ValidateSite(ctx context.Context) (*types.SiteValidationResponse, error) {
	graph, err := s.load(ctx, true)
	if err != nil {
		return nil, err
	}
	report := ops.AnalyzeSite(s.cfg, graph)
	return &types.SiteValidationResponse{
		BrokenMediaRefs:       append([]string(nil), report.BrokenMediaRefs...),
		BrokenInternalLinks:   append([]string(nil), report.BrokenInternalLinks...),
		MissingTemplates:      append([]string(nil), report.MissingTemplates...),
		OrphanedMedia:         append([]string(nil), report.OrphanedMedia...),
		DuplicateURLs:         append([]string(nil), report.DuplicateURLs...),
		DuplicateSlugs:        append([]string(nil), report.DuplicateSlugs...),
		TaxonomyInconsistency: append([]string(nil), report.TaxonomyInconsistency...),
		MessageCount: len(report.BrokenMediaRefs) +
			len(report.BrokenInternalLinks) +
			len(report.MissingTemplates) +
			len(report.OrphanedMedia) +
			len(report.DuplicateURLs) +
			len(report.DuplicateSlugs) +
			len(report.TaxonomyInconsistency),
	}, nil
}

func (s *Service) populateRuntimeGraphMetrics(ctx context.Context, status *types.RuntimeStatus) {
	if s == nil || status == nil {
		return
	}

	graph, err := s.load(ctx, true)
	if err != nil || graph == nil {
		return
	}

	status.Content.DocumentCount = len(graph.Documents)
	status.Content.RouteCount = len(graph.ByURL)
	status.Content.TaxonomyCount = len(graph.Taxonomies.Values)

	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		status.Content.ByType[doc.Type]++
		status.Content.ByLang[doc.Lang]++
		docStatus := strings.TrimSpace(doc.Status)
		if docStatus == "" {
			docStatus = "unknown"
		}
		status.Content.ByStatus[docStatus]++
	}

	for _, terms := range graph.Taxonomies.Values {
		status.Content.TaxonomyTermCount += len(terms)
	}

	report := ops.AnalyzeSite(s.cfg, graph)
	status.Integrity = types.RuntimeIntegrityStatus{
		BrokenMediaRefs:       len(report.BrokenMediaRefs),
		BrokenInternalLinks:   len(report.BrokenInternalLinks),
		MissingTemplates:      len(report.MissingTemplates),
		OrphanedMedia:         len(report.OrphanedMedia),
		DuplicateURLs:         len(report.DuplicateURLs),
		DuplicateSlugs:        len(report.DuplicateSlugs),
		TaxonomyInconsistency: len(report.TaxonomyInconsistency),
	}
}

func (s *Service) populateRuntimeStorageMetrics(status *types.RuntimeStatus) {
	if s == nil || s.cfg == nil || status == nil {
		return
	}

	largest := make([]types.RuntimeFileStat, 0, 5)
	status.Storage.ContentBytes = walkAndSummarizeDir(s.cfg.ContentDir, &largest, func(rel string, size int64) {
		if rel == "" {
			return
		}
		base := filepath.Base(rel)
		if lifecycle.IsDerivedPath(base) {
			status.Storage.DerivedBytes += size
			if strings.Contains(base, ".version.") {
				status.Storage.DerivedVersionCount++
			}
			if strings.Contains(base, ".trash.") {
				status.Storage.DerivedTrashCount++
			}
		}
	})
	status.Storage.PublicBytes = walkAndSummarizeDir(s.cfg.PublicDir, &largest, nil)

	for _, collection := range media.SupportedCollections {
		root, ok := runtimeMediaRoot(s.cfg, collection)
		if !ok {
			continue
		}
		walkAndSummarizeDir(root, nil, func(rel string, size int64) {
			if rel == "" {
				return
			}
			name := filepath.Base(rel)
			if strings.HasSuffix(name, ".meta.yaml") || lifecycle.IsDerivedPath(name) {
				return
			}
			status.Storage.MediaCounts[collection]++
			status.Storage.MediaBytes[collection] += size
			status.Content.MediaCounts[collection]++
		})
	}

	sort.Slice(largest, func(i, j int) bool {
		if largest[i].SizeBytes == largest[j].SizeBytes {
			return largest[i].Path < largest[j].Path
		}
		return largest[i].SizeBytes > largest[j].SizeBytes
	})
	if len(largest) > 5 {
		largest = largest[:5]
	}
	status.Storage.LargestFiles = largest
}

func (s *Service) populateRuntimeActivityMetrics(status *types.RuntimeStatus, now time.Time) {
	if s == nil || s.cfg == nil || status == nil {
		return
	}

	status.Activity.ActiveSessions = countActiveSessions(s.cfg.Admin.SessionStoreFile, now)

	s.lockMu.Lock()
	locks, err := s.loadLocksLocked(now)
	s.lockMu.Unlock()
	if err == nil {
		status.Activity.ActiveDocumentLocks = len(locks)
	}

	items, err := audit.List(s.cfg, 200)
	if err != nil {
		return
	}
	windowStart := now.Add(-runtimeAuditWindow)
	for _, entry := range items {
		if entry.Timestamp.IsZero() || entry.Timestamp.Before(windowStart) {
			continue
		}
		status.Activity.RecentAuditEvents++
		action := strings.TrimSpace(entry.Action)
		if action == "" {
			action = "unknown"
		}
		status.Activity.RecentAuditByAction[action]++
		if strings.EqualFold(action, "login") && strings.EqualFold(strings.TrimSpace(entry.Outcome), "fail") {
			status.Activity.RecentFailedLogins++
		}
	}
}

func (s *Service) populateRuntimeBuildStatus(status *types.RuntimeStatus) {
	if s == nil || s.cfg == nil || status == nil {
		return
	}
	path := filepath.Join(s.cfg.DataDir, "admin", "build-report.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var report ops.BuildReport
	if err := json.Unmarshal(body, &report); err != nil {
		return
	}
	status.LastBuild = &types.RuntimeBuildStatus{
		GeneratedAt:   report.GeneratedAt,
		Environment:   report.Environment,
		Target:        report.Target,
		Preview:       report.Preview,
		DocumentCount: report.DocumentCount,
		RouteCount:    report.RouteCount,
		PrepareMS:     report.Stats.Prepare.Milliseconds(),
		AssetsMS:      report.Stats.Assets.Milliseconds(),
		DocumentsMS:   report.Stats.Documents.Milliseconds(),
		TaxonomiesMS:  report.Stats.Taxonomies.Milliseconds(),
		SearchMS:      report.Stats.Search.Milliseconds(),
	}
}

func countActiveSessions(path string, now time.Time) int {
	if strings.TrimSpace(path) == "" {
		return 0
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var file runtimeSessionFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return 0
	}
	count := 0
	for _, session := range file.Sessions {
		if strings.TrimSpace(session.Token) == "" || now.After(session.ExpiresAt) {
			continue
		}
		count++
	}
	return count
}

func walkAndSummarizeDir(root string, largest *[]types.RuntimeFileStat, onFile func(rel string, size int64)) int64 {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return 0
	}

	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size := info.Size()
		total += size

		rel, relErr := filepath.Rel(".", path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		if onFile != nil {
			onFile(rel, size)
		}
		if largest != nil {
			*largest = append(*largest, types.RuntimeFileStat{
				Path:      rel,
				SizeBytes: size,
			})
		}
		return nil
	})
	return total
}

func runtimeMediaRoot(cfg *config.Config, collection string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	switch strings.TrimSpace(collection) {
	case "images":
		return filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir), true
	case "videos":
		return filepath.Join(cfg.ContentDir, cfg.Content.VideoDir), true
	case "audio":
		return filepath.Join(cfg.ContentDir, cfg.Content.AudioDir), true
	case "documents":
		return filepath.Join(cfg.ContentDir, cfg.Content.DocumentsDir), true
	case "uploads":
		return filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir), true
	case "assets":
		return filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir), true
	default:
		return "", false
	}
}
