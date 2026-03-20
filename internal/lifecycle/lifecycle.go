package lifecycle

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const TimestampFormat = "20060102T150405Z"

var derivedStemRE = regexp.MustCompile(`^(.*)\.(version|trash)\.(\d{8}T\d{6}Z)$`)

type State string

const (
	StateCurrent State = "current"
	StateVersion State = "version"
	StateTrash   State = "trash"
)

func IsDerivedPath(path string) bool {
	_, _, ok := ParsePath(path)
	return ok
}

func IsVersionPath(path string) bool {
	_, state, ok := ParsePath(path)
	return ok && state == StateVersion
}

func IsTrashPath(path string) bool {
	_, state, ok := ParsePath(path)
	return ok && state == StateTrash
}

func ParsePath(path string) (string, State, bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	stem, ext := splitStemAndExt(base)
	match := derivedStemRE.FindStringSubmatch(stem)
	if len(match) != 4 {
		return "", "", false
	}
	original := filepath.Join(dir, match[1]+ext)
	switch match[2] {
	case "version":
		return original, StateVersion, true
	case "trash":
		return original, StateTrash, true
	default:
		return "", "", false
	}
}

func BuildVersionPath(path string, now time.Time) string {
	return buildDerivedPath(path, "version", now)
}

func BuildTrashPath(path string, now time.Time) string {
	return buildDerivedPath(path, "trash", now)
}

func buildDerivedPath(path, kind string, now time.Time) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	stem, ext := splitStemAndExt(base)
	if original, _, ok := ParsePath(path); ok {
		base = filepath.Base(original)
		stem, ext = splitStemAndExt(base)
	}
	return filepath.Join(dir, stem+"."+kind+"."+now.UTC().Format(TimestampFormat)+ext)
}

func splitStemAndExt(base string) (string, string) {
	if strings.HasSuffix(base, ".meta.yaml") {
		core := strings.TrimSuffix(base, ".meta.yaml")
		ext := filepath.Ext(core)
		if ext == "" {
			return core, ".meta.yaml"
		}
		return strings.TrimSuffix(core, ext), ext + ".meta.yaml"
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base, ""
	}
	return strings.TrimSuffix(base, ext), ext
}

func OriginalPath(path string) string {
	if original, _, ok := ParsePath(path); ok {
		return original
	}
	return path
}

func ValidateCurrentPath(path string) error {
	if IsDerivedPath(path) {
		return fmt.Errorf("lifecycle-managed derived file paths are not valid current paths: %s", filepath.Base(path))
	}
	return nil
}
