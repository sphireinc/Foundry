package tests

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalContainsOnlyGoCode(t *testing.T) {
	root := ".."
	internalDir := filepath.Join(root, "internal")
	var violations []string

	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "testdata" || name == "fixtures" {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				violations = append(violations, rel)
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(d.Name()) != ".go" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal tree: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("internal/ must be Go-code-only; move fixtures to tests/fixtures or runtime state to project-root data/: %s", strings.Join(violations, ", "))
	}
}
