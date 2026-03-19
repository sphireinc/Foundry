package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := UnmarshalYAML(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.ApplyDefaults()

	return &cfg, nil
}

func UnmarshalYAML(b []byte, cfg *Config) error {
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return err
	}
	cfg.ApplyDefaults()
	if errs := Validate(cfg); len(errs) > 0 {
		return errs[0]
	}
	return nil
}
