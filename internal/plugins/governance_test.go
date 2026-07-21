package plugins

import (
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestEnforceGovernanceRequiresApprovedArtifactAndLicense(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "catalog", "name: catalog\nversion: 1.2.3\nlicense: Apache-2.0\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "catalog")
	meta, err := LoadMetadata(root, "catalog")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	digest, err := ArtifactSHA256(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatalf("digest plugin: %v", err)
	}
	policy := GovernancePolicy{
		Managed:         true,
		AllowedLicenses: []string{"Apache-2.0"},
		Approved: []config.ManagedPluginApproval{{
			Name: "catalog", Version: "1.2.3", SHA256: digest,
		}},
	}
	if err := EnforceGovernance(meta, policy); err != nil {
		t.Fatalf("expected approved plugin to pass governance: %v", err)
	}

	policy.Approved[0].SHA256 = "bad"
	if err := EnforceGovernance(meta, policy); err == nil {
		t.Fatal("expected mismatched artifact digest to be rejected")
	}
	policy.Approved[0].SHA256 = digest
	policy.AllowedLicenses = []string{"MIT"}
	if err := EnforceGovernance(meta, policy); err == nil {
		t.Fatal("expected disallowed license to be rejected")
	}
}

func TestEnforceGovernanceRequiresStrictRPCWhenConfigured(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "strict", "name: strict\nversion: 1.0.0\nlicense: MIT\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "strict")
	meta, err := LoadMetadata(root, "strict")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	digest, err := ArtifactSHA256(meta.Directory)
	if err != nil {
		t.Fatalf("digest plugin: %v", err)
	}
	err = EnforceGovernance(meta, GovernancePolicy{
		Managed:              true,
		RequireStrictSandbox: true,
		Approved:             []config.ManagedPluginApproval{{Name: meta.Name, Version: meta.Version, SHA256: digest}},
	})
	if err == nil {
		t.Fatal("expected in-process plugin to fail strict runtime policy")
	}
}

func TestNewManagerWithGovernanceRejectsUnapprovedPlugin(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "unapproved", "name: unapproved\nversion: 1.0.0\nlicense: MIT\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "unapproved")
	if _, err := NewManagerWithGovernance(root, []string{"unapproved"}, GovernancePolicy{Managed: true}); err == nil {
		t.Fatal("expected managed startup to reject an unapproved plugin")
	}
}
