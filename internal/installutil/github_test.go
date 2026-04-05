package installutil

import (
	"strings"
	"testing"
)

func TestNormalizeGitHubInstallURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "shorthand", raw: "acme/plugin", want: "https://github.com/acme/plugin.git"},
		{name: "https adds git suffix", raw: "https://github.com/acme/plugin", want: "https://github.com/acme/plugin.git"},
		{name: "git stays git", raw: "git@github.com:acme/plugin.git", want: "git@github.com:acme/plugin.git"},
		{name: "non github stays as-is", raw: "https://gitlab.com/acme/plugin", want: "https://gitlab.com/acme/plugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeGitHubInstallURL(tt.raw); got != tt.want {
				t.Fatalf("NormalizeGitHubInstallURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestValidateGitHubInstallURL(t *testing.T) {
	validateName := func(name string) (string, error) {
		if strings.Contains(name, "/") || strings.Contains(name, "..") || strings.TrimSpace(name) == "" {
			return "", assertErr("invalid name")
		}
		return name, nil
	}

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "https repo", raw: "https://github.com/acme/plugin", want: "https://github.com/acme/plugin.git"},
		{name: "git ssh repo", raw: "git@github.com:acme/plugin.git", want: "git@github.com:acme/plugin.git"},
		{name: "reject empty ok", raw: "", want: ""},
		{name: "reject non github", raw: "https://gitlab.com/acme/plugin", wantErr: "must target github.com"},
		{name: "reject invalid path", raw: "https://github.com/acme", wantErr: "owner/repository"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateGitHubInstallURL("plugin", tt.raw, validateName)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ValidateGitHubInstallURL(%q) error = %v, want substring %q", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateGitHubInstallURL(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ValidateGitHubInstallURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestInferRepoName(t *testing.T) {
	validateName := func(name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", assertErr("empty")
		}
		return name, nil
	}

	got, err := InferRepoName("git@github.com:acme/plugin.git", "plugin", validateName)
	if err != nil {
		t.Fatalf("InferRepoName git ssh: %v", err)
	}
	if got != "plugin" {
		t.Fatalf("InferRepoName git ssh = %q, want plugin", got)
	}

	got, err = InferRepoName("https://github.com/acme/plugin.git", "plugin", validateName)
	if err != nil {
		t.Fatalf("InferRepoName https: %v", err)
	}
	if got != "plugin" {
		t.Fatalf("InferRepoName https = %q, want plugin", got)
	}
}

func TestRepoZipURL(t *testing.T) {
	got, err := RepoZipURL("git@github.com:acme/plugin.git")
	if err != nil {
		t.Fatalf("RepoZipURL git ssh: %v", err)
	}
	if got != "https://github.com/acme/plugin/archive/refs/heads/main.zip" {
		t.Fatalf("unexpected zip URL: %q", got)
	}

	if _, err := RepoZipURL("https://gitlab.com/acme/plugin.git"); err == nil {
		t.Fatal("expected non-github zip URL to fail")
	}
}

func TestSafeArchivePath(t *testing.T) {
	root := t.TempDir()

	got, err := SafeArchivePath(root, "plugin-main/plugin.yaml")
	if err != nil {
		t.Fatalf("SafeArchivePath valid: %v", err)
	}
	if !strings.HasPrefix(got, root) {
		t.Fatalf("expected %q to stay under %q", got, root)
	}

	for _, name := range []string{"../escape", "/abs/path"} {
		if _, err := SafeArchivePath(root, name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
