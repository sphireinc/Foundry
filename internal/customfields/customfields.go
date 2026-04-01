package customfields

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"gopkg.in/yaml.v3"
)

type Store struct {
	Values map[string]any `yaml:"values"`
}

func Path(cfg *config.Config) string {
	contentDir := "content"
	if cfg != nil && strings.TrimSpace(cfg.ContentDir) != "" {
		contentDir = cfg.ContentDir
	}
	return filepath.Join(contentDir, "custom-fields.yaml")
}

func Load(cfg *config.Config) (*Store, error) {
	path := Path(cfg)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Values: map[string]any{}}, nil
		}
		return nil, err
	}
	var store Store
	if err := yaml.Unmarshal(body, &store); err != nil {
		return nil, err
	}
	if store.Values == nil {
		store.Values = map[string]any{}
	}
	return &store, nil
}

func Save(cfg *config.Config, store *Store) error {
	if store == nil {
		store = &Store{}
	}
	if store.Values == nil {
		store.Values = map[string]any{}
	}
	body, err := yaml.Marshal(store)
	if err != nil {
		return err
	}
	path := Path(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func NormalizeValues(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func ExtractGroup(values map[string]any, key string) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	group, ok := values[strings.TrimSpace(key)]
	if !ok {
		return map[string]any{}
	}
	return NormalizeValues(group)
}

func SetGroup(values map[string]any, key string, group map[string]any) map[string]any {
	if values == nil {
		values = map[string]any{}
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return values
	}
	values[key] = group
	return values
}

func ValidateRoot(values map[string]any) error {
	if values == nil {
		return nil
	}
	for key, value := range values {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("custom field groups must not use empty keys")
		}
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("custom field group %q must be an object", key)
		}
	}
	return nil
}
