package backup

import (
	"errors"
	"path/filepath"
	"strings"
)

var errOutsideRoot = errors.New("path outside root")

func filepathAbs(path string) (string, error) {
	return filepath.Abs(path)
}

func filepathRel(root, target string) (string, error) {
	return filepath.Rel(root, target)
}

func stringsHasDotDotPrefix(rel string) bool {
	return strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
