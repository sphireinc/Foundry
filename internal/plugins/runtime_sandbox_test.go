package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictExecutableContainedPaths(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "bin", "plugin")
	// #nosec G306 -- executable permission is required for the validation fixture.
	if err := os.WriteFile(executable, []byte("executable fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("bin/plugin", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"bin/plugin", executable, filepath.Join(canonicalRoot, "bin", "plugin"), "alias"} {
		t.Run(command, func(t *testing.T) {
			meta := Metadata{Directory: root}
			meta.Runtime.Command = []string{command}
			gotRoot, gotPath, err := strictExecutable(meta)
			if err != nil {
				t.Fatal(err)
			}
			if gotRoot != canonicalRoot || gotPath != filepath.Join(canonicalRoot, "bin", "plugin") {
				t.Fatalf("unexpected executable: %s %s", gotRoot, gotPath)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(root, "not-executable"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	external := filepath.Join(outside, "plugin")
	// #nosec G306 -- executable permission is required for the escape fixture.
	if err := os.WriteFile(external, []byte("external fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{nil, {""}, {"../plugin"}, {external}, {root + "-sibling/plugin"}, {"escape"}, {"outside/plugin"}, {"bin"}, {"not-executable"}} {
		meta := Metadata{Directory: root}
		meta.Runtime.Command = command
		if _, _, err := strictExecutable(meta); err == nil {
			t.Fatalf("unsafe command accepted: %v", command)
		}
	}
}
