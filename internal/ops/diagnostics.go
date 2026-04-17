package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/media"
	"github.com/sphireinc/foundry/internal/theme"
)

var (
	markdownLinkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	htmlHrefRE     = regexp.MustCompile(`(?i)\bhref="([^"]+)"`)
	htmlSrcRE      = regexp.MustCompile(`(?i)\bsrc="([^"]+)"`)
)

// DiagnosticReport aggregates site-level validation findings discovered outside
// the strict config/theme validators.
type DiagnosticReport struct {
	BrokenMediaRefs       []string
	BrokenInternalLinks   []string
	MissingTemplates      []string
	OrphanedMedia         []string
	DuplicateURLs         []string
	DuplicateSlugs        []string
	TaxonomyInconsistency []string
}

// Messages flattens all diagnostic categories into a single ordered message
// slice suitable for CLI output.
func (r DiagnosticReport) Messages() []string {
	var out []string
	out = append(out, r.DuplicateURLs...)
	out = append(out, r.DuplicateSlugs...)
	out = append(out, r.BrokenMediaRefs...)
	out = append(out, r.BrokenInternalLinks...)
	out = append(out, r.MissingTemplates...)
	out = append(out, r.OrphanedMedia...)
	out = append(out, r.TaxonomyInconsistency...)
	return out
}

// AnalyzeSite inspects the loaded graph and supporting files for operational
// issues such as broken references, missing templates, and orphaned media.
func AnalyzeSite(cfg *config.Config, graph *content.SiteGraph) DiagnosticReport {
	report := DiagnosticReport{}
	if cfg == nil || graph == nil {
		return report
	}

	seenURLs := make(map[string]string)
	seenSlugs := make(map[string]string)
	referencedMedia := make(map[string]struct{})
	allowedTaxonomies := make(map[string]struct{})
	for _, name := range cfg.Taxonomies.DefaultSet {
		allowedTaxonomies[strings.TrimSpace(name)] = struct{}{}
	}
	for name := range cfg.Taxonomies.Definitions {
		allowedTaxonomies[strings.TrimSpace(name)] = struct{}{}
	}

	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		if other, ok := seenURLs[doc.URL]; ok {
			report.DuplicateURLs = append(report.DuplicateURLs, fmt.Sprintf("duplicate URL %s for %s and %s", doc.URL, other, doc.SourcePath))
		} else {
			seenURLs[doc.URL] = doc.SourcePath
		}

		slugKey := doc.Type + "|" + doc.Lang + "|" + doc.Slug
		if other, ok := seenSlugs[slugKey]; ok {
			report.DuplicateSlugs = append(report.DuplicateSlugs, fmt.Sprintf("duplicate slug within type/lang %q for %s and %s", slugKey, other, doc.SourcePath))
		} else {
			seenSlugs[slugKey] = doc.SourcePath
		}

		for name := range doc.Taxonomies {
			if _, ok := allowedTaxonomies[name]; !ok {
				report.TaxonomyInconsistency = append(report.TaxonomyInconsistency, fmt.Sprintf("document %s uses unknown taxonomy %s", doc.SourcePath, name))
			}
		}

		layoutPath := theme.NewManager(cfg.ThemesDir, cfg.Theme).LayoutPath(doc.Layout)
		if strings.TrimSpace(layoutPath) == "" {
			report.MissingTemplates = append(report.MissingTemplates, fmt.Sprintf("document %s uses invalid layout %s", doc.SourcePath, doc.Layout))
		} else if _, err := os.Stat(layoutPath); err != nil {
			report.MissingTemplates = append(report.MissingTemplates, fmt.Sprintf("document %s is missing layout template %s", doc.SourcePath, doc.Layout))
		}

		for _, ref := range collectReferences(doc.RawBody) {
			switch {
			case strings.HasPrefix(ref, media.ReferenceScheme):
				if err := validateMediaReference(cfg, ref); err != nil {
					report.BrokenMediaRefs = append(report.BrokenMediaRefs, fmt.Sprintf("document %s has broken media reference %s: %v", doc.SourcePath, ref, err))
				} else {
					resolved, _ := media.ResolveReference(ref)
					referencedMedia[resolved.Collection+"/"+resolved.Path] = struct{}{}
				}
			case isInternalLink(ref):
				if err := validateInternalReference(cfg, ref, seenURLs); err != nil {
					report.BrokenInternalLinks = append(report.BrokenInternalLinks, fmt.Sprintf("document %s has broken internal link %s: %v", doc.SourcePath, ref, err))
				}
			}
		}
	}

	for _, orphan := range findOrphanedMedia(cfg, referencedMedia) {
		report.OrphanedMedia = append(report.OrphanedMedia, orphan)
	}

	return report
}

func collectReferences(raw string) []string {
	refs := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			refs = append(refs, value)
		}
	}
	for _, match := range markdownLinkRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	for _, match := range htmlHrefRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	for _, match := range htmlSrcRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	return refs
}

func isInternalLink(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.HasPrefix(ref, "/") && !strings.HasPrefix(ref, "//")
}

func validateMediaReference(cfg *config.Config, ref string) error {
	resolved, err := media.ResolveReference(ref)
	if err != nil {
		return err
	}
	root := mediaRoot(cfg, resolved.Collection)
	if root == "" {
		return fmt.Errorf("unknown media collection")
	}
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(resolved.Path)))
	return err
}

func validateInternalReference(cfg *config.Config, ref string, seen map[string]string) error {
	ref = strings.TrimSpace(ref)
	if idx := strings.Index(ref, "#"); idx >= 0 {
		ref = ref[:idx]
	}
	if ref == "" {
		return nil
	}
	if _, ok := seen[ref]; ok {
		return nil
	}
	if ref == cfg.Feed.RSSPath || ref == cfg.Feed.SitemapPath || ref == "/search.json" || ref == "/preview-links.json" {
		return nil
	}
	switch {
	case strings.HasPrefix(ref, "/images/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, strings.TrimPrefix(ref, "/images/")))
		return err
	case strings.HasPrefix(ref, "/videos/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.VideoDir, strings.TrimPrefix(ref, "/videos/")))
		return err
	case strings.HasPrefix(ref, "/audio/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.AudioDir, strings.TrimPrefix(ref, "/audio/")))
		return err
	case strings.HasPrefix(ref, "/documents/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.DocumentsDir, strings.TrimPrefix(ref, "/documents/")))
		return err
	case strings.HasPrefix(ref, "/uploads/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir, strings.TrimPrefix(ref, "/uploads/")))
		return err
	case strings.HasPrefix(ref, "/assets/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, strings.TrimPrefix(ref, "/assets/")))
		return err
	default:
		return fmt.Errorf("route not found")
	}
}

func findOrphanedMedia(cfg *config.Config, referenced map[string]struct{}) []string {
	orphaned := make([]string, 0)
	for _, collection := range []string{"images", "videos", "audio", "documents", "uploads", "assets"} {
		root := mediaRoot(cfg, collection)
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if strings.HasSuffix(rel, ".meta.yaml") || lifecycle.IsDerivedPath(rel) {
				return nil
			}
			key := collection + "/" + rel
			if _, ok := referenced[key]; !ok {
				orphaned = append(orphaned, fmt.Sprintf("orphaned media %s", filepath.ToSlash(filepath.Join(cfg.ContentDir, mediaSubdir(cfg, collection), rel))))
			}
			return nil
		})
	}
	return orphaned
}

func mediaRoot(cfg *config.Config, collection string) string {
	subdir := mediaSubdir(cfg, collection)
	if subdir == "" {
		return ""
	}
	return filepath.Join(cfg.ContentDir, subdir)
}

func mediaSubdir(cfg *config.Config, collection string) string {
	switch collection {
	case "images":
		return cfg.Content.ImagesDir
	case "videos":
		return cfg.Content.VideoDir
	case "audio":
		return cfg.Content.AudioDir
	case "documents":
		return cfg.Content.DocumentsDir
	case "uploads":
		return cfg.Content.UploadsDir
	case "assets":
		return cfg.Content.AssetsDir
	default:
		return ""
	}
}
