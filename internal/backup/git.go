package backup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

type GitSnapshot struct {
	RepoDir   string
	Revision  string
	CreatedAt time.Time
	Message   string
	Changed   bool
	Pushed    bool
	RemoteURL string
	Branch    string
}

func CreateGitSnapshot(cfg *config.Config, message string, push bool) (*GitSnapshot, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	repoDir := filepath.Join(cfg.Backup.Dir, "git")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return nil, err
	}
	if err := ensureGitRepo(repoDir); err != nil {
		return nil, err
	}
	remoteURL := strings.TrimSpace(cfg.Backup.GitRemoteURL)
	branch := strings.TrimSpace(cfg.Backup.GitBranch)
	if branch == "" {
		branch = "main"
	}
	if remoteURL != "" {
		if err := ensureGitRemote(repoDir, remoteURL); err != nil {
			return nil, err
		}
	}
	if err := syncContentWorkingTree(repoDir, cfg.ContentDir); err != nil {
		return nil, err
	}
	if strings.TrimSpace(message) == "" {
		message = "Foundry content snapshot " + time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := gitOutput(repoDir, "add", "-A"); err != nil {
		return nil, err
	}
	changed, err := gitHasStagedChanges(repoDir)
	if err != nil {
		return nil, err
	}
	if !changed {
		rev, _ := gitOutput(repoDir, "rev-parse", "HEAD")
		return &GitSnapshot{
			RepoDir:   repoDir,
			Revision:  strings.TrimSpace(rev),
			CreatedAt: time.Now().UTC(),
			Message:   message,
			Changed:   false,
			RemoteURL: remoteURL,
			Branch:    branch,
		}, nil
	}
	if _, err := gitOutputWithConfig(repoDir, []string{"-c", "user.name=Foundry", "-c", "user.email=foundry@localhost", "commit", "-m", message}); err != nil {
		return nil, err
	}
	rev, err := gitOutput(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	logInfo, err := gitOutput(repoDir, "log", "-1", "--pretty=format:%cI")
	if err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(logInfo))
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	snapshot := &GitSnapshot{
		RepoDir:   repoDir,
		Revision:  strings.TrimSpace(rev),
		CreatedAt: createdAt,
		Message:   message,
		Changed:   true,
		RemoteURL: remoteURL,
		Branch:    branch,
	}
	if remoteURL != "" && (push || cfg.Backup.GitPushOnChange) {
		if err := pushGitSnapshot(repoDir, branch); err != nil {
			return nil, err
		}
		snapshot.Pushed = true
	}
	return snapshot, nil
}

func ListGitSnapshots(cfg *config.Config, limit int) ([]GitSnapshot, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	repoDir := filepath.Join(cfg.Backup.Dir, "git")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out, err := gitOutput(repoDir, "log", fmt.Sprintf("-%d", limit), "--pretty=format:%H|%cI|%s")
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	items := make([]GitSnapshot, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		createdAt, _ := time.Parse(time.RFC3339, parts[1])
		items = append(items, GitSnapshot{
			RepoDir:   repoDir,
			Revision:  parts[0],
			CreatedAt: createdAt,
			Message:   parts[2],
			Changed:   true,
			RemoteURL: strings.TrimSpace(cfg.Backup.GitRemoteURL),
			Branch:    strings.TrimSpace(cfg.Backup.GitBranch),
		})
	}
	return items, nil
}

func ensureGitRepo(repoDir string) error {
	if info, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil && info.IsDir() {
		return nil
	}
	_, err := gitOutput(repoDir, "init")
	return err
}

func ensureGitRemote(repoDir, remoteURL string) error {
	current, err := gitOutput(repoDir, "remote", "get-url", "origin")
	if err == nil {
		if strings.TrimSpace(current) == strings.TrimSpace(remoteURL) {
			return nil
		}
		_, err = gitOutput(repoDir, "remote", "set-url", "origin", remoteURL)
		return err
	}
	_, err = gitOutput(repoDir, "remote", "add", "origin", remoteURL)
	return err
}

func pushGitSnapshot(repoDir, branch string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	_, err := gitOutput(repoDir, "push", "-u", "origin", "HEAD:"+branch)
	return err
}

func syncContentWorkingTree(repoDir, contentDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(repoDir, entry.Name())); err != nil {
			return err
		}
	}
	target := filepath.Join(repoDir, filepath.Base(contentDir))
	return copyDir(contentDir, target)
}

func copyDir(source, target string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		input, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, input, info.Mode())
	})
}

func gitHasStagedChanges(repoDir string) (bool, error) {
	cmd := exec.Command("git", "-C", repoDir, "diff", "--cached", "--quiet")
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func gitOutput(repoDir string, args ...string) (string, error) {
	return gitOutputWithConfig(repoDir, args)
}

func gitOutputWithConfig(repoDir string, args []string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
