package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/lifecycle"
)

func (s *Service) versionFile(path string, now time.Time) error {
	if err := lifecycle.ValidateCurrentPath(path); err != nil {
		return err
	}
	if _, err := s.fs.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	versionPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return lifecycle.BuildVersionPath(path, ts)
	}, now)
	if err != nil {
		return err
	}
	if err := s.fs.Rename(path, versionPath); err != nil {
		return err
	}
	return s.pruneVersions(path)
}

func (s *Service) snapshotDocumentVersion(path string, now time.Time, comment, actor string) error {
	if err := lifecycle.ValidateCurrentPath(path); err != nil {
		return err
	}
	raw, err := s.fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	fm, body, err := content.ParseDocument(raw)
	if err != nil {
		return err
	}
	comment = strings.TrimSpace(comment)
	if comment != "" {
		if fm.Params == nil {
			fm.Params = make(map[string]any)
		}
		fm.Params["version_comment"] = comment
		fm.Params["versioned_at"] = now.UTC().Format(time.RFC3339)
	}
	actor = strings.TrimSpace(actor)
	if actor != "" {
		if fm.Params == nil {
			fm.Params = make(map[string]any)
		}
		fm.Params["version_actor"] = actor
	}
	versionPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return lifecycle.BuildVersionPath(path, ts)
	}, now)
	if err != nil {
		return err
	}
	rendered, err := marshalDocument(fm, body)
	if err != nil {
		return err
	}
	if err := s.fs.WriteFile(versionPath, rendered, 0o644); err != nil {
		return err
	}
	return s.pruneVersions(path)
}

func (s *Service) trashFile(path string, now time.Time) (string, error) {
	if err := lifecycle.ValidateCurrentPath(path); err != nil {
		return "", err
	}
	if _, err := s.fs.Stat(path); err != nil {
		return "", err
	}
	trashPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return lifecycle.BuildTrashPath(path, ts)
	}, now)
	if err != nil {
		return "", err
	}
	if err := s.fs.Rename(path, trashPath); err != nil {
		return "", err
	}
	return trashPath, nil
}

func (s *Service) pruneVersions(currentPath string) error {
	maxVersions := s.cfg.Content.MaxVersionsPerFile
	if maxVersions <= 0 {
		return nil
	}
	dir := filepath.Dir(currentPath)
	entries, err := s.fs.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type candidate struct {
		name string
		path string
	}
	candidates := make([]candidate, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		original, state, ok := lifecycle.ParsePath(fullPath)
		if !ok || state != lifecycle.StateVersion {
			continue
		}
		if filepath.Clean(original) != filepath.Clean(currentPath) {
			continue
		}
		candidates = append(candidates, candidate{name: entry.Name(), path: fullPath})
	}
	if len(candidates) <= maxVersions {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].name > candidates[j].name
	})
	for _, candidate := range candidates[maxVersions:] {
		if err := s.fs.Remove(candidate.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Service) uniqueDerivedPath(build func(time.Time) string, now time.Time) (string, error) {
	for i := 0; i < 10_000; i++ {
		candidate := build(now.Add(time.Duration(i) * time.Second))
		if _, err := s.fs.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}
	return "", os.ErrExist
}
