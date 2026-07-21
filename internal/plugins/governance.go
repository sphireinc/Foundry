package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

// GovernancePolicy constrains the plugins a managed deployment may run. It is
// deliberately separate from plugin.yaml: a plugin cannot approve itself.
type GovernancePolicy struct {
	Managed              bool
	Approved             []config.ManagedPluginApproval
	AllowedLicenses      []string
	RequireStrictSandbox bool
}

func GovernancePolicyFromConfig(cfg *config.Config) GovernancePolicy {
	if cfg == nil || !cfg.ManagedRuntimeEnabled() {
		return GovernancePolicy{}
	}
	policy := cfg.Foundry.Managed.PluginPolicy
	return GovernancePolicy{
		Managed:              true,
		Approved:             append([]config.ManagedPluginApproval(nil), policy.Approved...),
		AllowedLicenses:      append([]string(nil), policy.AllowedLicenses...),
		RequireStrictSandbox: policy.RequireStrictSandbox,
	}
}

// EnforceGovernance validates the installed artifact against the deployment's
// trusted policy. Empty approval lists intentionally allow no managed plugins.
func EnforceGovernance(meta Metadata, policy GovernancePolicy) error {
	if !policy.Managed {
		return nil
	}
	if err := ValidateInstalledPlugin(filepath.Dir(meta.Directory), meta.Name); err != nil {
		return err
	}
	if err := EnsureRuntimeSupported(meta); err != nil {
		return err
	}
	if policy.RequireStrictSandbox && (meta.Runtime.Mode != "rpc" || meta.Runtime.Sandbox.Profile != "strict") {
		return fmt.Errorf("plugin %q must use the strict RPC runtime required by this managed deployment", meta.Name)
	}
	if len(policy.AllowedLicenses) > 0 && !containsNormalized(policy.AllowedLicenses, meta.License) {
		return fmt.Errorf("plugin %q license %q is not approved for this managed deployment", meta.Name, meta.License)
	}
	digest, err := ArtifactSHA256(meta.Directory)
	if err != nil {
		return err
	}
	for _, approval := range policy.Approved {
		if strings.TrimSpace(approval.Name) != meta.Name || strings.TrimSpace(approval.Version) != meta.Version {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(approval.SHA256), digest) {
			return nil
		}
		return fmt.Errorf("plugin %q version %q does not match its approved artifact digest", meta.Name, meta.Version)
	}
	return fmt.Errorf("plugin %q version %q is not approved for this managed deployment", meta.Name, meta.Version)
}

func GovernanceReviewStatus(meta Metadata, policy GovernancePolicy) string {
	if !policy.Managed {
		return "self-managed"
	}
	if err := EnforceGovernance(meta, policy); err != nil {
		return "blocked"
	}
	return "approved"
}

// ArtifactSHA256 returns a deterministic digest for plugin files. VCS and
// rollback directories are excluded because they are not runnable artifacts.
func ArtifactSHA256(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("plugin artifact directory cannot be empty")
	}
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".rollback" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk plugin artifact: %w", err)
	}
	sort.Strings(files)
	h := sha256.New()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(h, filepath.ToSlash(rel)+"\n"); err != nil {
			return "", err
		}
		// #nosec G304 -- path is a regular file returned by WalkDir below root.
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if _, err := io.WriteString(h, "\n"); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func containsNormalized(values []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return false
	}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}
