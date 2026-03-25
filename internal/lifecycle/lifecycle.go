package lifecycle

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TimestampFormat is the sortable UTC timestamp format used in lifecycle-managed
// filenames such as foo.version.<timestamp>.md and foo.trash.<timestamp>.md.
const TimestampFormat = "20060102T150405Z"

var derivedStemRE = regexp.MustCompile(`^(.*)\.(version|trash)\.(\d{8}T\d{6}Z)$`)

// State identifies whether a lifecycle-managed path is current, versioned, or
// trashed.
type State string

const (
	StateCurrent State = "current"
	StateVersion State = "version"
	StateTrash   State = "trash"
)

// IsDerivedPath reports whether path is a lifecycle-managed version or trash
// file rather than the current canonical file.
func IsDerivedPath(path string) bool {
	_, _, ok := ParsePath(path)
	return ok
}

// IsVersionPath reports whether path is a retained previous version.
func IsVersionPath(path string) bool {
	_, state, ok := ParsePath(path)
	return ok && state == StateVersion
}

// IsTrashPath reports whether path is a soft-deleted file.
func IsTrashPath(path string) bool {
	_, state, ok := ParsePath(path)
	return ok && state == StateTrash
}

// ParsePath resolves a lifecycle-managed path back to its canonical current
// path and lifecycle state.
func ParsePath(path string) (string, State, bool) {
	original, state, _, ok := ParsePathDetails(path)
	return original, state, ok
}

// ParsePathDetails resolves a lifecycle-managed path back to its canonical
// current path, lifecycle state, and timestamp.
func ParsePathDetails(path string) (string, State, time.Time, bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	stem, ext := splitStemAndExt(base)
	match := derivedStemRE.FindStringSubmatch(stem)
	if len(match) != 4 {
		return "", "", time.Time{}, false
	}
	original := filepath.Join(dir, match[1]+ext)
	ts, err := time.Parse(TimestampFormat, match[3])
	if err != nil {
		return "", "", time.Time{}, false
	}
	switch match[2] {
	case "version":
		return original, StateVersion, ts, true
	case "trash":
		return original, StateTrash, ts, true
	default:
		return "", "", time.Time{}, false
	}
}

// BuildVersionPath returns the versioned filename for the current path at now.
func BuildVersionPath(path string, now time.Time) string {
	return buildDerivedPath(path, "version", now)
}

// BuildTrashPath returns the trash filename for the current path at now.
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

// OriginalPath returns the canonical current path for either a current or
// lifecycle-derived path.
func OriginalPath(path string) string {
	if original, _, ok := ParsePath(path); ok {
		return original
	}
	return path
}

// ValidateCurrentPath rejects version/trash paths where a canonical current
// path is required.
func ValidateCurrentPath(path string) error {
	if IsDerivedPath(path) {
		return fmt.Errorf("lifecycle-managed derived file paths are not valid current paths: %s", filepath.Base(path))
	}
	return nil
}
