package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	Environment  string
	Target       string
	OverlayPaths []string
}

func Load(path string) (*Config, error) {
	return LoadWithOptions(path, LoadOptions{})
}

func LoadWithOptions(path string, opts LoadOptions) (*Config, error) {
	root, err := loadConfigNode(path)
	if err != nil {
		return nil, err
	}

	overlayPaths := make([]string, 0, len(opts.OverlayPaths)+1)
	if env := strings.TrimSpace(opts.Environment); env != "" && env != "default" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		candidate := filepath.Join(filepath.Dir(path), base+"."+env+filepath.Ext(path))
		if _, err := os.Stat(candidate); err == nil {
			overlayPaths = append(overlayPaths, candidate)
		}
	}
	overlayPaths = append(overlayPaths, opts.OverlayPaths...)

	for _, overlayPath := range overlayPaths {
		if strings.TrimSpace(overlayPath) == "" {
			continue
		}
		overlay, err := loadConfigNode(overlayPath)
		if err != nil {
			return nil, err
		}
		root = mergeNodes(root, overlay)
	}

	var cfg Config
	b, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal merged config: %w", err)
	}
	if err := UnmarshalYAML(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	target := strings.TrimSpace(opts.Target)
	if target == "" {
		target = strings.TrimSpace(cfg.Deploy.DefaultTarget)
	}
	if target != "" {
		if err := cfg.ApplyDeployTarget(target); err != nil {
			return nil, fmt.Errorf("apply deploy target: %w", err)
		}
	}
	if env := strings.TrimSpace(opts.Environment); env != "" {
		cfg.Environment = env
	}
	cfg.ApplyDefaults()

	return &cfg, nil
}

func UnmarshalYAML(b []byte, cfg *Config) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return err
	}
	cfg.Admin.localOnlySet = yamlMappingPathExists(doc.Content, "admin", "local_only")
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return err
	}
	cfg.ApplyDefaults()
	if errs := Validate(cfg); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func yamlMappingPathExists(nodes []*yaml.Node, path ...string) bool {
	if len(nodes) == 0 || len(path) == 0 {
		return false
	}
	node := nodes[0]
	for _, part := range path {
		if node == nil || node.Kind != yaml.MappingNode {
			return false
		}
		next := -1
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == part {
				next = i + 1
				break
			}
		}
		if next < 0 {
			return false
		}
		node = node.Content[next]
	}
	return true
}

func loadConfigNode(path string) (*yaml.Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	}
	return doc.Content[0], nil
}

func mergeNodes(base, overlay *yaml.Node) *yaml.Node {
	if base == nil {
		return cloneNode(overlay)
	}
	if overlay == nil {
		return cloneNode(base)
	}
	if base.Kind != yaml.MappingNode || overlay.Kind != yaml.MappingNode {
		return cloneNode(overlay)
	}

	merged := cloneNode(base)
	index := make(map[string]int)
	for i := 0; i+1 < len(merged.Content); i += 2 {
		index[merged.Content[i].Value] = i
	}

	for i := 0; i+1 < len(overlay.Content); i += 2 {
		key := overlay.Content[i]
		value := overlay.Content[i+1]
		if existing, ok := index[key.Value]; ok {
			merged.Content[existing+1] = mergeNodes(merged.Content[existing+1], value)
			continue
		}
		merged.Content = append(merged.Content, cloneNode(key), cloneNode(value))
	}

	return merged
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	cloned := *node
	if len(node.Content) > 0 {
		cloned.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			cloned.Content[i] = cloneNode(child)
		}
	}
	return &cloned
}
