package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

const (
	e2eTempPrefix = "foundry-e2e-"
)

func main() {
	ctx, stop := signalContext()
	defer stop()

	root, err := os.Getwd()
	if err != nil {
		fatalf("get working directory: %v", err)
	}

	tempRoot, err := os.MkdirTemp("", e2eTempPrefix)
	if err != nil {
		fatalf("create temp workspace: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	tempContent := filepath.Join(tempRoot, "content")
	tempData := filepath.Join(tempRoot, "data")
	tempPublic := filepath.Join(tempRoot, "public")
	tempBackups := filepath.Join(tempRoot, ".foundry", "backups")
	tempOverlay := filepath.Join(tempRoot, "site.e2e.overlay.yaml")

	if err := copyDir(filepath.Join(root, "content"), tempContent); err != nil {
		fatalf("copy content: %v", err)
	}
	for _, dir := range []string{tempData, tempPublic, tempBackups} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("create temp directory %s: %v", dir, err)
		}
	}

	overlay := fmt.Sprintf(`content_dir: %q
public_dir: %q
data_dir: %q
backup:
  dir: %q
admin:
  users_file: %q
  session_store_file: %q
  lock_file: %q
`, tempContent, tempPublic, tempData, tempBackups,
		filepath.Join(tempContent, "config", "admin-users.yaml"),
		filepath.Join(tempData, "admin", "sessions.yaml"),
		filepath.Join(tempData, "admin", "locks.yaml"),
	)
	if err := os.WriteFile(tempOverlay, []byte(overlay), 0o644); err != nil {
		fatalf("write e2e config overlay: %v", err)
	}

	args := []string{"run", "./cmd/foundry", "--config-overlay", tempOverlay, "serve"}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("run foundry e2e server: %v", err)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	if runtime.GOOS == "windows" {
		return context.WithCancel(context.Background())
	}
	return signalNotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

var signalNotifyContext = func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	return signalNotifyContextImpl(parent, signals...)
}

func signalNotifyContextImpl(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signalNotify(ch, signals...)
	go func() {
		select {
		case <-ctx.Done():
		case <-ch:
			cancel()
		}
		signalStop(ch)
		close(ch)
	}()
	return ctx, cancel
}

var signalNotify = func(ch chan<- os.Signal, signals ...os.Signal) {
	signal.Notify(ch, signals...)
}

var signalStop = func(ch chan<- os.Signal) {
	signal.Stop(ch)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
