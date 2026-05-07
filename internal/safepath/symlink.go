package safepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureNoSymlinkEscape rejects paths that resolve outside root after symlink
// resolution and also rejects symlinked path components inside the root.
//
// It is intended for security-sensitive filesystem reads and writes where a
// lexical path check is not sufficient.
func EnsureNoSymlinkEscape(root, target string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("root path cannot be empty")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	ok, err := IsWithinRoot(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("path must stay inside %s", root)
	}

	if info, err := os.Lstat(rootAbs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path must stay inside %s after resolving symlinks", root)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must stay inside %s", root)
	}
	if rel == "." {
		return nil
	}

	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				ok, err := IsWithinRoot(rootAbs, current)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("path must stay inside %s", root)
				}
				continue
			}
			return err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return err
		}
		ok, err := IsWithinRoot(rootAbs, resolved)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("path must stay inside %s after resolving symlinks", root)
		}
	}

	return nil
}
