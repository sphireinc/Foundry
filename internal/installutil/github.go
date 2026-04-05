package installutil

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func NormalizeGitHubInstallURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "git@") {
		return raw
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil && strings.EqualFold(u.Host, "github.com") {
			u.Path = strings.TrimSuffix(u.Path, "/")
			if !strings.HasSuffix(u.Path, ".git") {
				u.Path += ".git"
			}
			return u.String()
		}
		return raw
	}
	if strings.Count(raw, "/") == 1 {
		return "https://github.com/" + raw + ".git"
	}
	return raw
}

func ValidateGitHubInstallURL(kind, raw string, validateName func(string) (string, error)) (string, error) {
	normalized := NormalizeGitHubInstallURL(raw)
	if strings.TrimSpace(normalized) == "" {
		return "", nil
	}
	if strings.HasPrefix(normalized, "git@github.com:") {
		name, err := InferRepoName(normalized, kind, validateName)
		if err != nil {
			return "", err
		}
		if _, err := validateName(name); err != nil {
			return "", fmt.Errorf("invalid GitHub repository path: %w", err)
		}
		return normalized, nil
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("parse %s URL: %w", kind, err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("%s URL must use https or git@github.com", kind)
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("%s URL must target github.com", kind)
	}

	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("%s URL must point to a GitHub owner/repository", kind)
	}
	if _, err := validateName(parts[1]); err != nil {
		return "", fmt.Errorf("invalid GitHub repository path: %w", err)
	}
	return normalized, nil
}

func InferRepoName(raw, kind string, validateName func(string) (string, error)) (string, error) {
	if strings.HasPrefix(raw, "git@") {
		idx := strings.Index(raw, ":")
		if idx >= 0 && idx+1 < len(raw) {
			path := strings.Trim(raw[idx+1:], "/")
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			name = strings.TrimSpace(name)
			if name != "" {
				return validateName(name)
			}
		}
		return "", fmt.Errorf("could not infer %s name from URL", kind)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse %s URL: %w", kind, err)
	}
	path := strings.Trim(strings.TrimSpace(u.Path), "/")
	if path == "" {
		return "", fmt.Errorf("could not infer %s name from URL", kind)
	}
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("could not infer %s name from URL", kind)
	}
	return validateName(name)
}

func RepoZipURL(repoURL string) (string, error) {
	if strings.HasPrefix(repoURL, "git@github.com:") {
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
		if strings.Count(path, "/") != 1 {
			return "", fmt.Errorf("zip fallback requires a GitHub owner/repo path")
		}
		return fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", path), nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("zip fallback currently supports GitHub only")
	}
	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	return fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", path), nil
}

func DownloadAndExtractRepoArchive(client *http.Client, repoURL, targetDir, tempPrefix, kind string, maxBytes int64) error {
	zipURL, err := RepoZipURL(repoURL)
	if err != nil {
		return err
	}
	resp, err := client.Get(zipURL)
	if err != nil {
		return fmt.Errorf("download %s zip: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", tempPrefix+"-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if written > maxBytes {
		return fmt.Errorf("%s zip exceeds %d bytes", kind, maxBytes)
	}
	_ = tmpFile.Close()

	zr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer zr.Close()

	tempDir, err := os.MkdirTemp("", tempPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip contains unsupported symlink entry: %s", f.Name)
		}
		fp, err := SafeArchivePath(tempDir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(fp)
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = rc.Close()
			_ = out.Close()
			return err
		}
		_ = rc.Close()
		_ = out.Close()
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("zip extraction failed")
	}
	root := filepath.Join(tempDir, entries[0].Name())
	rootInfo, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("zip extraction failed: root entry is not a directory")
	}
	return os.Rename(root, targetDir)
}

func StripVCSMetadata(targetDir string) error {
	for _, rel := range []string{".git", ".gitmodules"} {
		path := filepath.Join(targetDir, rel)
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove VCS metadata %q: %w", rel, err)
		}
	}
	return nil
}

func SafeArchivePath(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("zip contains empty entry name")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("zip entry escapes target dir: %s", name)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(name)))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rootWithSep := rootAbs + string(filepath.Separator)
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootWithSep) {
		return "", fmt.Errorf("zip entry escapes target dir: %s", name)
	}
	return targetAbs, nil
}
