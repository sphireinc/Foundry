package media

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/safepath"
	"github.com/yuin/goldmark"
	rendererhtml "github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

var encodedURLBytes = regexp.MustCompile(`(?:%[0-9a-fA-F]{2})+`)

type AuditAsset struct {
	Asset
	UsedBy []string `json:"used_by"`
}
type AuditReport struct {
	Assets           []AuditAsset         `json:"assets"`
	PotentialOrphans []string             `json:"potential_orphans"`
	Accessibility    []AccessibilityIssue `json:"accessibility"`
	Notes            []string             `json:"notes"`
}

// Audit is a non-destructive, conservative reference scan. Dynamic theme/plugin
// references cannot be proven absent, so candidates are never deleted automatically.
func Audit(cfg *config.Config) (*AuditReport, error) {
	inventory, err := Inventory(cfg)
	if err != nil {
		return nil, err
	}
	report := &AuditReport{Assets: []AuditAsset{}, PotentialOrphans: []string{}, Accessibility: []AccessibilityIssue{}, Notes: []string{"Potential orphans are review candidates, not proof of non-use. Dynamic references, external consumers and concatenated URLs may be invisible. No files were changed; use media trash/restore only after manual review."}}
	for _, asset := range inventory {
		report.Assets = append(report.Assets, AuditAsset{Asset: asset, UsedBy: []string{}})
	}
	documentRoots := []string{}
	for _, directory := range []string{cfg.Content.PagesDir, cfg.Content.PostsDir} {
		root, err := safepath.ResolveRelativeUnderRoot(cfg.ContentDir, directory)
		if err != nil {
			return nil, err
		}
		documentRoots = append(documentRoots, root)
	}
	roots := []string{cfg.ContentDir, cfg.DataDir, filepath.Join(cfg.ThemesDir, cfg.Theme), cfg.PluginsDir}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		if err := safepath.EnsureNoSymlinkEscape(root, root); err != nil {
			return nil, err
		}
		scoped, openErr := os.OpenRoot(root)
		if os.IsNotExist(openErr) {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		err = filepath.WalkDir(root, func(filename string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlinked audit source is not allowed: %s", filename)
			}
			absolute, err := filepath.Abs(filename)
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[absolute] || lifecycle.IsDerivedPath(filename) || strings.HasSuffix(strings.ToLower(filename), ".meta.yaml") {
				return nil
			}
			seen[absolute] = true
			switch strings.ToLower(filepath.Ext(filename)) {
			case ".md", ".html", ".htm", ".css", ".js", ".json", ".yaml", ".yml", ".toml", ".go", ".svg", ".xml":
			default:
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("audit source is not a regular file: %s", filename)
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() > 16<<20 {
				return fmt.Errorf("audit source exceeds 16 MiB: %s", filename)
			}
			// #nosec G304 -- source is a regular file in a symlink-rejecting project walk.
			relative, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			body, err := readScopedFile(scoped, relative, 16<<20)
			if err != nil {
				return err
			}
			scanReferences(report, filename, body)
			isDocument := false
			for _, documentRoot := range documentRoots {
				within, err := safepath.IsWithinRoot(documentRoot, filename)
				if err != nil {
					return err
				}
				isDocument = isDocument || within
			}
			if isDocument && strings.EqualFold(filepath.Ext(filename), ".md") {
				var metadata map[string]any
				markdown, err := frontmatter.Parse(bytes.NewReader(body), &metadata)
				if err != nil {
					return fmt.Errorf("parse audit document %s: %w", filename, err)
				}
				var rendered bytes.Buffer
				if err := goldmark.New(goldmark.WithRendererOptions(rendererhtml.WithUnsafe())).Convert(markdown, &rendered); err != nil {
					return err
				}
				enriched, err := EnrichHTML(rendered.Bytes(), cfg, nil)
				if err != nil {
					return err
				}
				report.Accessibility = append(report.Accessibility, CheckHTML(enriched, filename)...)
			}
			return nil
		})
		_ = scoped.Close()
		if err != nil {
			return nil, err
		}
	}
	// Effective configuration covers selected overlays and custom config filenames.
	configBody, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	scanReferences(report, "effective configuration", configBody)
	assetsRoot, err := CollectionRoot(cfg, "assets")
	if err != nil {
		return nil, err
	}
	cssRoot := filepath.Join(assetsRoot, "css")
	for i := range report.Assets {
		asset := &report.Assets[i]
		within, err := safepath.IsWithinRoot(cssRoot, asset.SourcePath)
		if err != nil {
			return nil, err
		}
		// The asset pipeline bundles content CSS independently of CopyAssets
		// and without requiring an explicit reference to each input file.
		if within && strings.EqualFold(filepath.Ext(asset.SourcePath), ".css") {
			asset.UsedBy = append(asset.UsedBy, "generated /assets/css/foundry.bundle.css")
		}
	}
	for _, asset := range report.Assets {
		if len(asset.UsedBy) == 0 {
			report.PotentialOrphans = append(report.PotentialOrphans, asset.Reference)
		}
	}
	return report, nil
}
func scanReferences(report *AuditReport, filename string, body []byte) {
	text := stdhtml.UnescapeString(string(body))
	if unescaped, err := url.PathUnescape(text); err == nil {
		text = unescaped
	} else {
		// CSS percentages and unrelated malformed escapes must not hide valid URLs.
		text = encodedURLBytes.ReplaceAllStringFunc(text, func(value string) string {
			decoded, err := url.PathUnescape(value)
			if err != nil {
				return value
			}
			return decoded
		})
	}
	text = strings.ReplaceAll(text, `\/`, "/")
	for i := range report.Assets {
		asset := &report.Assets[i]
		// Substring matching errs on the side of retaining files. It also covers
		// Markdown closing punctuation, srcset, CSS URLs and JSON/YAML values.
		if strings.Contains(text, asset.Reference) || strings.Contains(text, strings.TrimPrefix(asset.PublicURL, "/")) {
			alreadySeen := false
			for _, existing := range asset.UsedBy {
				if existing == filename {
					alreadySeen = true
					break
				}
			}
			if !alreadySeen {
				asset.UsedBy = append(asset.UsedBy, filename)
			}
		}
	}
}
