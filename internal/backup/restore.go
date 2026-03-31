package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

func RestoreZipSnapshot(cfg *config.Config, source string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("backup source must not be empty")
	}
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	contentDir := cfg.ContentDir
	contentBase := filepath.Base(contentDir)
	restoreRoot := filepath.Join(filepath.Dir(contentDir), ".foundry-restore-"+time.Now().UTC().Format("20060102-150405.000"))
	extractedContentRoot := filepath.Join(restoreRoot, contentBase)

	if err := os.MkdirAll(restoreRoot, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(restoreRoot)

	foundContent := false
	for _, file := range reader.File {
		name := filepath.Clean(filepath.FromSlash(file.Name))
		if name == "." || name == "backup-manifest.json" {
			continue
		}
		if !strings.HasPrefix(name, contentBase+string(filepath.Separator)) {
			continue
		}
		foundContent = true
		target := filepath.Join(restoreRoot, name)
		if err := ensureWithinRoot(restoreRoot, target); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			return err
		}
		if err := out.Close(); err != nil {
			_ = rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	if !foundContent {
		return fmt.Errorf("backup archive does not contain %q", contentBase)
	}
	if _, err := os.Stat(extractedContentRoot); err != nil {
		return fmt.Errorf("restored content root missing: %w", err)
	}

	if _, err := CreateManagedSnapshot(cfg); err != nil {
		return fmt.Errorf("pre-restore snapshot failed: %w", err)
	}

	backupOld := contentDir + ".restore-old-" + time.Now().UTC().Format("20060102-150405.000")
	if err := os.Rename(contentDir, backupOld); err != nil {
		return err
	}
	if err := os.Rename(extractedContentRoot, contentDir); err != nil {
		_ = os.Rename(backupOld, contentDir)
		return err
	}
	return os.RemoveAll(backupOld)
}

func ensureWithinRoot(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("archive entry escapes restore root")
	}
	return nil
}
