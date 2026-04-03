package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	adminusers "github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	e2ePrefix  = "e2e-"
	configPath = "content/config/site.yaml"
)

func main() {
	cfg, err := config.Load(configPath)
	if err != nil {
		fatalf("load config: %v", err)
	}

	if err := cleanupContent(cfg.ContentDir); err != nil {
		fatalf("cleanup content: %v", err)
	}
	if err := cleanupBackups(cfg.Backup.Dir); err != nil {
		fatalf("cleanup backups: %v", err)
	}
	if err := cleanupUsers(cfg.Admin.UsersFile); err != nil {
		fatalf("cleanup users: %v", err)
	}
	if err := cleanupLocks(cfg.Admin.LockFile); err != nil {
		fatalf("cleanup locks: %v", err)
	}
	if err := cleanupAudit(filepath.Join(cfg.DataDir, "admin", "audit.jsonl")); err != nil {
		fatalf("cleanup audit: %v", err)
	}
}

func cleanupContent(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if !strings.Contains(strings.ToLower(name), e2ePrefix) {
			return nil
		}
		if d.IsDir() {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return filepath.SkipDir
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
}

func cleanupBackups(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, e2ePrefix) {
			continue
		}
		target := filepath.Join(dir, entry.Name())
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func cleanupUsers(path string) error {
	entries, err := adminusers.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	filtered := make([]adminusers.User, 0, len(entries))
	changed := false
	for _, entry := range entries {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(entry.Username)), e2ePrefix) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !changed {
		return nil
	}
	return adminusers.Save(path, filtered)
}

type lockFile struct {
	Locks []map[string]any `yaml:"locks"`
}

func cleanupLocks(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file lockFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return err
	}
	filtered := make([]map[string]any, 0, len(file.Locks))
	changed := false
	for _, entry := range file.Locks {
		sourcePath, _ := entry["source_path"].(string)
		if strings.Contains(strings.ToLower(strings.TrimSpace(sourcePath)), e2ePrefix) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !changed {
		return nil
	}
	file.Locks = filtered
	out, err := yaml.Marshal(&file)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func cleanupAudit(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := bytes.Split(body, []byte{'\n'})
	filtered := make([][]byte, 0, len(lines))
	changed := false
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if strings.Contains(strings.ToLower(string(line)), e2ePrefix) {
			changed = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !changed {
		return nil
	}
	if len(filtered) == 0 {
		return os.WriteFile(path, []byte{}, 0o644)
	}
	out := bytes.Join(filtered, []byte{'\n'})
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
