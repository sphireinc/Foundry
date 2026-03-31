package backup

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

type Snapshot struct {
	Path         string
	SizeBytes    int64
	CreatedAt    time.Time
	SourceBytes  int64
	FreeBytes    uint64
	RequiredFree uint64
}

type manifest struct {
	CreatedAt       time.Time `json:"created_at"`
	ContentDir      string    `json:"content_dir"`
	SourceBytes     int64     `json:"source_bytes"`
	FreeBytes       uint64    `json:"free_bytes"`
	RequiredFree    uint64    `json:"required_free_bytes"`
	HeadroomPercent int       `json:"headroom_percent"`
}

func CreateManagedSnapshot(cfg *config.Config) (*Snapshot, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := os.MkdirAll(cfg.Backup.Dir, 0o755); err != nil {
		return nil, err
	}
	target := nextManagedSnapshotPath(cfg.Backup.Dir, time.Now())
	snapshot, err := CreateZipSnapshot(cfg, target)
	if err != nil {
		return nil, err
	}
	if cfg.Backup.RetentionCount > 0 {
		if err := Prune(cfg.Backup.Dir, cfg.Backup.RetentionCount); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func CreateZipSnapshot(cfg *config.Config, target string) (*Snapshot, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("backup target must not be empty")
	}
	if filepath.Ext(target) == "" {
		target += ".zip"
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}

	sourceBytes, err := dirSize(cfg.ContentDir)
	if err != nil {
		return nil, err
	}
	freeBytes, err := freeSpace(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	required := requiredFreeBytes(sourceBytes, cfg.Backup.HeadroomPercent, cfg.Backup.MinFreeMB)
	if freeBytes < required {
		return nil, fmt.Errorf("not enough free space for backup: required %d bytes, available %d bytes", required, freeBytes)
	}

	tmpTarget := target + ".tmp"
	_ = os.Remove(tmpTarget)

	file, err := os.Create(tmpTarget)
	if err != nil {
		return nil, err
	}

	createdAt := time.Now().UTC()
	zw := zip.NewWriter(file)
	writeErr := addPathToZip(zw, cfg.ContentDir, cfg.ContentDir)
	if writeErr == nil {
		writeErr = writeManifest(zw, manifest{
			CreatedAt:       createdAt,
			ContentDir:      cfg.ContentDir,
			SourceBytes:     sourceBytes,
			FreeBytes:       freeBytes,
			RequiredFree:    required,
			HeadroomPercent: cfg.Backup.HeadroomPercent,
		})
	}
	closeErr := zw.Close()
	fileCloseErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(tmpTarget)
		return nil, writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpTarget)
		return nil, closeErr
	}
	if fileCloseErr != nil {
		_ = os.Remove(tmpTarget)
		return nil, fileCloseErr
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		_ = os.Remove(tmpTarget)
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Path:         target,
		SizeBytes:    info.Size(),
		CreatedAt:    createdAt,
		SourceBytes:  sourceBytes,
		FreeBytes:    freeBytes,
		RequiredFree: required,
	}, nil
}

func List(dir string) ([]Snapshot, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("backup dir must not be empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Snapshot, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".zip" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, Snapshot{
			Path:      filepath.Join(dir, entry.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func Prune(dir string, retain int) error {
	if retain < 0 {
		return fmt.Errorf("retain must not be negative")
	}
	items, err := List(dir)
	if err != nil {
		return err
	}
	for idx, item := range items {
		if idx < retain {
			continue
		}
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func PathIsUnderBackupRoot(cfg *config.Config, candidate string) bool {
	if cfg == nil || strings.TrimSpace(cfg.Backup.Dir) == "" || strings.TrimSpace(candidate) == "" {
		return false
	}
	backupRoot, err := filepath.Abs(cfg.Backup.Dir)
	if err != nil {
		return false
	}
	check, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(backupRoot, check)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func requiredFreeBytes(sourceBytes int64, headroomPercent int, minFreeMB int64) uint64 {
	if headroomPercent < 100 {
		headroomPercent = 100
	}
	if minFreeMB < 0 {
		minFreeMB = 0
	}
	base := uint64(sourceBytes)
	headroom := base * uint64(headroomPercent) / 100
	withBuffer := base + uint64(minFreeMB)*1024*1024
	if withBuffer > headroom {
		return withBuffer
	}
	return headroom
}

func generatedName(now time.Time) string {
	return fmt.Sprintf("content-backup-%s.zip", now.UTC().Format("20060102-150405.000000000"))
}

func nextManagedSnapshotPath(dir string, now time.Time) string {
	base := generatedName(now)
	target := filepath.Join(dir, base)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}
	stamp := now.UTC().Format("20060102-150405.000000000")
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("content-backup-%s-%02d.zip", stamp, i))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func addPathToZip(zw *zip.Writer, root, source string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(root), current)
		if err != nil {
			return err
		}
		writer, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

func writeManifest(zw *zip.Writer, data manifest) error {
	writer, err := zw.Create("backup-manifest.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(writer)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
