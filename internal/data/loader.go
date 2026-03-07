package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadDir(root string) (*Store, error) {
	store := New()

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("stat data dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("data path is not a directory: %s", root)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk data dir: %w", walkErr)
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".yaml", ".yml", ".json":
		default:
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}

		key := normalizeKey(rel)
		val, err := loadFile(path, ext)
		if err != nil {
			return fmt.Errorf("load data file %s: %w", path, err)
		}

		store.Set(key, val)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return store, nil
}

func loadFile(path, ext string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var v any
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("unmarshal yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("unmarshal json: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	return v, nil
}

func normalizeKey(rel string) string {
	rel = filepath.ToSlash(rel)
	ext := filepath.Ext(rel)
	rel = strings.TrimSuffix(rel, ext)
	return rel
}
