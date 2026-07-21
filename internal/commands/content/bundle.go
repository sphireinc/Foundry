package contentcmd

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/safepath"
	"github.com/sphireinc/foundry/internal/theme"
	"gopkg.in/yaml.v3"
)

const contentBundleManifest = "foundry-content-manifest.yaml"

const maxContentBundleFileBytes = 64 << 20

type contentBundle struct {
	Format  string              `yaml:"format"`
	Version int                 `yaml:"version"`
	Theme   string              `yaml:"theme"`
	Files   []contentBundleFile `yaml:"files"`
}

type contentBundleFile struct {
	Path   string `yaml:"path"`
	SHA256 string `yaml:"sha256"`
}

func runExport(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry content export <bundle.zip>")
	}
	return exportContentBundle(cfg, strings.TrimSpace(args[3]))
}

func exportContentBundle(cfg *config.Config, target string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if target == "" {
		return fmt.Errorf("bundle path must not be empty")
	}
	if filepath.Ext(target) == "" {
		target += ".zip"
	}
	files, err := bundleFiles(cfg.ContentDir)
	if err != nil {
		return err
	}
	manifest := contentBundle{Format: "foundry-content-bundle", Version: 1, Theme: cfg.Theme, Files: make([]contentBundleFile, 0, len(files))}
	for _, source := range files {
		rel, err := filepath.Rel(cfg.ContentDir, source)
		if err != nil {
			return err
		}
		digest, err := fileSHA256(source)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, contentBundleFile{Path: filepath.ToSlash(filepath.Join("content", rel)), SHA256: digest})
	}
	body, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	tmp := target + ".tmp"
	defer func() { _ = os.Remove(tmp) }()
	// #nosec G304 -- tmp is derived from the caller-provided export target.
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	if err := writeZipFile(zw, contentBundleManifest, body); err != nil {
		_ = zw.Close()
		_ = out.Close()
		return err
	}
	for i, source := range files {
		if err := writeZipPath(zw, manifest.Files[i].Path, source); err != nil {
			_ = zw.Close()
			_ = out.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	fmt.Printf("exported Foundry content bundle to %s\n", target)
	return nil
}

func importContentBundle(cfg *config.Config, source string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if source == "" {
		return fmt.Errorf("bundle path must not be empty")
	}
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	stage, err := os.MkdirTemp(filepath.Dir(cfg.ContentDir), ".foundry-content-import-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	manifest, err := extractBundle(reader.File, stage)
	if err != nil {
		return err
	}
	if manifest.Theme != "" && manifest.Theme != cfg.Theme {
		if err := theme.NewManager(cfg.ThemesDir, manifest.Theme).MustExist(); err != nil {
			return fmt.Errorf("bundle requires theme %q: %w", manifest.Theme, err)
		}
		return fmt.Errorf("bundle uses theme %q but this site uses %q; switch themes before importing", manifest.Theme, cfg.Theme)
	}
	if err := validateBundleFiles(stage, manifest); err != nil {
		return err
	}
	stagedContent := filepath.Join(stage, "content")
	if _, err := os.Stat(stagedContent); err != nil {
		return fmt.Errorf("bundle does not contain content: %w", err)
	}
	backup := cfg.ContentDir + ".import-rollback-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	if err := os.Rename(cfg.ContentDir, backup); err != nil {
		return err
	}
	if err := os.Rename(stagedContent, cfg.ContentDir); err != nil {
		_ = os.Rename(backup, cfg.ContentDir)
		return fmt.Errorf("apply content bundle: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("content bundle applied but cleanup failed: %w", err)
	}
	fmt.Printf("imported Foundry content bundle from %s\n", source)
	return nil
}

func bundleFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == "config/admin-users.yaml" {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("content export does not support non-regular file %q", rel)
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func extractBundle(files []*zip.File, stage string) (contentBundle, error) {
	var manifest contentBundle
	for _, file := range files {
		if file.UncompressedSize64 > maxContentBundleFileBytes {
			return manifest, fmt.Errorf("bundle entry %q exceeds the %d byte limit", file.Name, maxContentBundleFileBytes)
		}
		name := filepath.ToSlash(filepath.Clean(file.Name))
		if name == contentBundleManifest {
			body, err := readZipFile(file)
			if err != nil {
				return manifest, err
			}
			if err := yaml.Unmarshal(body, &manifest); err != nil {
				return manifest, err
			}
			continue
		}
		if !strings.HasPrefix(name, "content/") || strings.Contains(name, "../") || file.Mode()&os.ModeSymlink != 0 {
			return manifest, fmt.Errorf("unsafe bundle entry %q", file.Name)
		}
		target, err := safepath.ResolveRelativeUnderRoot(stage, name)
		if err != nil {
			return manifest, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		body, err := readZipFile(file)
		if err != nil {
			return manifest, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return manifest, err
		}
		// #nosec G306 -- imported content is intentionally readable by the CMS runtime.
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return manifest, err
		}
	}
	if manifest.Format != "foundry-content-bundle" || manifest.Version != 1 {
		return manifest, fmt.Errorf("unsupported content bundle format")
	}
	return manifest, nil
}

func validateBundleFiles(stage string, manifest contentBundle) error {
	if len(manifest.Files) == 0 {
		return fmt.Errorf("content bundle has no files")
	}
	for _, entry := range manifest.Files {
		if !strings.HasPrefix(entry.Path, "content/") || strings.Contains(entry.Path, "../") {
			return fmt.Errorf("unsafe manifest path %q", entry.Path)
		}
		path, err := safepath.ResolveRelativeUnderRoot(stage, entry.Path)
		if err != nil {
			return err
		}
		digest, err := fileSHA256(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(digest, entry.SHA256) {
			return fmt.Errorf("content bundle checksum mismatch for %q", entry.Path)
		}
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name string, body []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetMode(0o644)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}
func writeZipPath(zw *zip.Writer, name, source string) error {
	// #nosec G304 -- source was enumerated under cfg.ContentDir by bundleFiles.
	body, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeZipFile(zw, name, body)
}
func readZipFile(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
func fileSHA256(path string) (string, error) {
	// #nosec G304 -- path is validated as a root-bounded staged bundle file.
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
