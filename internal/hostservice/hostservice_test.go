package hostservice

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceNameIsStableAndSanitized(t *testing.T) {
	projectDir := filepath.Join("/tmp", "My Site")
	got := ServiceName(projectDir)
	if !strings.HasPrefix(got, "foundry-my-site-") {
		t.Fatalf("unexpected service name: %q", got)
	}
	if got != ServiceName(projectDir) {
		t.Fatalf("service name should be stable")
	}
}
