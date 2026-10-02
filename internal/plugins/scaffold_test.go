package plugins

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldPluginContracts(t *testing.T) {
	for _, mode := range []string{"rpc", "compiled"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path, err := Scaffold(root, "test-plugin", mode)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateInstalledPlugin(root, "test-plugin"); err != nil {
				t.Fatal(err)
			}
			meta, err := LoadMetadata(root, "test-plugin")
			if err != nil {
				t.Fatal(err)
			}
			if len(AnalyzeInstalled(meta).Mismatches) != 0 {
				t.Fatal("starter permissions do not match code")
			}
			if mode == "rpc" && (meta.Runtime.Mode != "rpc" || meta.Runtime.Command[0] != "./bin/test-plugin") {
				t.Fatal("RPC starter does not execute its built artifact")
			}
			count := 0
			if err := filepath.WalkDir(path, func(file string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && filepath.Ext(file) == ".go" {
					count++
					if _, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.AllErrors); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("expected source and runnable test, got %d Go files", count)
			}
			before, err := os.ReadFile(filepath.Join(path, "plugin.yaml")) // #nosec G304 -- test-owned scaffold path.
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Scaffold(root, "test-plugin", mode); err == nil {
				t.Fatal("existing plugin overwritten")
			}
			after, err := os.ReadFile(filepath.Join(path, "plugin.yaml")) // #nosec G304 -- test-owned scaffold path.
			if err != nil || string(before) != string(after) {
				t.Fatal("existing manifest changed")
			}
		})
	}
}

func TestScaffoldRejectsUnsafeNamesAndModes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../escape", "bad/name", "Bad", "two words", "name\nkey: true", ".hidden", "foo;bar"} {
		if _, err := Scaffold(root, name, "rpc"); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := Scaffold(root, "okay", "unknown"); err == nil {
		t.Fatal("unsupported runtime accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid arguments created files")
	}
	if !strings.Contains(pluginStarterReadme("test", "compiled"), "internal packages") {
		t.Fatal("compiled module restriction missing")
	}
}
