package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadYAMLDocument(path string) (*yaml.Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid config document")
	}

	return &doc, nil
}

func SaveYAMLDocument(path string, doc *yaml.Node) error {
	if doc == nil {
		return fmt.Errorf("yaml document is nil")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func UpsertTopLevelScalar(path, key, value string) error {
	doc, err := LoadYAMLDocument(path)
	if err != nil {
		return err
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config root must be a mapping")
	}

	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		v := root.Content[i+1]

		if k.Value == key {
			v.Kind = yaml.ScalarNode
			v.Tag = "!!str"
			v.Value = value
			return SaveYAMLDocument(path, doc)
		}
	}

	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)

	return SaveYAMLDocument(path, doc)
}

func RemoveTopLevelKey(path, key string) error {
	doc, err := LoadYAMLDocument(path)
	if err != nil {
		return err
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config root must be a mapping")
	}

	out := make([]*yaml.Node, 0, len(root.Content))
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		v := root.Content[i+1]
		if k.Value == key {
			continue
		}
		out = append(out, k, v)
	}
	root.Content = out
	return SaveYAMLDocument(path, doc)
}

func UpsertNestedScalar(path string, keyPath []string, value string) error {
	doc, err := LoadYAMLDocument(path)
	if err != nil {
		return err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("config root must be a mapping")
	}
	if len(keyPath) == 0 {
		return fmt.Errorf("key path must not be empty")
	}

	current := doc.Content[0]
	for i, key := range keyPath {
		last := i == len(keyPath)-1
		var next *yaml.Node
		for j := 0; j < len(current.Content); j += 2 {
			k := current.Content[j]
			v := current.Content[j+1]
			if k.Value == key {
				next = v
				break
			}
		}
		if next == nil {
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if last {
				next = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
			}
			current.Content = append(current.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				next,
			)
		}
		if last {
			next.Kind = yaml.ScalarNode
			next.Tag = "!!str"
			next.Value = value
			next.Content = nil
			return SaveYAMLDocument(path, doc)
		}
		if next.Kind != yaml.MappingNode {
			next.Kind = yaml.MappingNode
			next.Tag = "!!map"
			next.Value = ""
			next.Content = nil
		}
		current = next
	}
	return SaveYAMLDocument(path, doc)
}

func EnsureStringListValue(path string, keyPath []string, value string) error {
	doc, err := LoadYAMLDocument(path)
	if err != nil {
		return err
	}

	seq, err := ensureSequenceAtPath(doc.Content[0], keyPath)
	if err != nil {
		return err
	}

	for _, item := range seq.Content {
		if item.Kind == yaml.ScalarNode && item.Value == value {
			return SaveYAMLDocument(path, doc)
		}
	}

	seq.Content = append(seq.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	})

	return SaveYAMLDocument(path, doc)
}

func RemoveStringListValue(path string, keyPath []string, value string) error {
	doc, err := LoadYAMLDocument(path)
	if err != nil {
		return err
	}

	seq, err := findSequenceAtPath(doc.Content[0], keyPath)
	if err != nil {
		return err
	}
	if seq == nil {
		return SaveYAMLDocument(path, doc)
	}

	out := make([]*yaml.Node, 0, len(seq.Content))
	for _, item := range seq.Content {
		if item.Kind == yaml.ScalarNode && item.Value == value {
			continue
		}
		out = append(out, item)
	}
	seq.Content = out

	return SaveYAMLDocument(path, doc)
}

func ensureSequenceAtPath(root *yaml.Node, keyPath []string) (*yaml.Node, error) {
	if root == nil {
		return nil, fmt.Errorf("root node is nil")
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a mapping")
	}
	if len(keyPath) == 0 {
		return nil, fmt.Errorf("key path must not be empty")
	}

	current := root
	for i, key := range keyPath {
		last := i == len(keyPath)-1

		var next *yaml.Node
		for j := 0; j < len(current.Content); j += 2 {
			k := current.Content[j]
			v := current.Content[j+1]
			if k.Value == key {
				next = v
				break
			}
		}

		if next == nil {
			var newVal *yaml.Node
			if last {
				newVal = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			} else {
				newVal = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}

			current.Content = append(current.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				newVal,
			)
			next = newVal
		}

		if last {
			if next.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%v must be a sequence", keyPath)
			}
			return next, nil
		}

		if next.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%v must contain a mapping at %q", keyPath, key)
		}
		current = next
	}

	return nil, fmt.Errorf("invalid key path")
}

func findSequenceAtPath(root *yaml.Node, keyPath []string) (*yaml.Node, error) {
	if root == nil {
		return nil, fmt.Errorf("root node is nil")
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a mapping")
	}
	if len(keyPath) == 0 {
		return nil, fmt.Errorf("key path must not be empty")
	}

	current := root
	for i, key := range keyPath {
		last := i == len(keyPath)-1

		var next *yaml.Node
		for j := 0; j < len(current.Content); j += 2 {
			k := current.Content[j]
			v := current.Content[j+1]
			if k.Value == key {
				next = v
				break
			}
		}

		if next == nil {
			return nil, nil
		}

		if last {
			if next.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%v must be a sequence", keyPath)
			}
			return next, nil
		}

		if next.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%v must contain a mapping at %q", keyPath, key)
		}
		current = next
	}

	return nil, nil
}
