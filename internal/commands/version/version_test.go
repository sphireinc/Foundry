package version

import (
	"strings"
	"testing"
)

func TestCommandMetadataAndString(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "version" || cmd.Group() == "" || cmd.RequiresConfig() {
		t.Fatalf("unexpected command metadata")
	}
	out := String()
	if !strings.Contains(out, "Foundry") || !strings.Contains(out, "commit:") {
		t.Fatalf("unexpected version string: %q", out)
	}
	if err := cmd.Run(nil, nil); err != nil {
		t.Fatalf("run version: %v", err)
	}
}
