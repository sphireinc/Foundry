package safepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidatePathComponent(kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", kind)
	}
	if filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be a single directory name", kind)
	}
	if strings.Contains(value, "/") || strings.Contains(value, `\`) {
		return "", fmt.Errorf("%s must be a single directory name", kind)
	}

	clean := filepath.Clean(value)
	if clean == "." || clean == ".." || clean != value {
		return "", fmt.Errorf("%s must be a single directory name", kind)
	}

	return value, nil
}

func ResolveRelativeUnderRoot(root, rel string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be relative")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside %s", root)
	}

	target := filepath.Join(rootAbs, clean)
	ok, err := IsWithinRoot(rootAbs, target)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("path must stay inside %s", root)
	}

	return target, nil
}

func IsWithinRoot(root, target string) (bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}

	rootWithSep := rootAbs + string(filepath.Separator)
	return targetAbs == rootAbs || strings.HasPrefix(targetAbs, rootWithSep), nil
}
