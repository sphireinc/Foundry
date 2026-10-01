package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Strict plugins must be prebuilt executables inside their artifact directory.
// Source runners need compiler caches, subprocesses and a wider filesystem view.
func strictExecutable(meta Metadata) (string, string, error) {
	root, err := filepath.Abs(meta.Directory)
	if err != nil {
		return "", "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	path := meta.Runtime.Command[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("strict RPC executable must be inside the plugin artifact directory")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", "", fmt.Errorf("strict RPC command must be a prebuilt executable")
	}
	return root, path, nil
}

func strictSandboxAvailable() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("strict RPC OS sandbox is currently supported only on macOS; refusing unsandboxed execution on %s", runtime.GOOS)
	}
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		return fmt.Errorf("strict RPC runtime requires /usr/bin/sandbox-exec: %w", err)
	}
	return nil
}

func rpcCommand(meta Metadata) (*exec.Cmd, error) {
	if meta.Runtime.Sandbox.Profile != "strict" {
		return exec.Command(meta.Runtime.Command[0], meta.Runtime.Command[1:]...), nil // #nosec G204 -- default RPC is explicitly trusted executable code.
	}
	if err := strictSandboxAvailable(); err != nil {
		return nil, err
	}
	root, executable, err := strictExecutable(meta)
	if err != nil {
		return nil, err
	}
	// Inherited stdio is the only writable channel. No network, file writes,
	// process forks, or execution of other programs is permitted. System reads
	// provide the dynamic loader and Go runtime without exposing host home/data.
	profile := `(version 1)
(deny default)
(allow process-exec (literal ` + strconv.Quote(executable) + `))
(allow file-read* (subpath ` + strconv.Quote(root) + `)
  (literal "/") (subpath "/System/Library") (subpath "/usr/lib")
  (subpath "/System/Cryptexes/OS")
  (subpath "/System/Volumes/Preboot/Cryptexes/OS")
  (literal "/dev/null") (literal "/dev/urandom") (literal "/dev/random"))
(allow sysctl-read)
(allow mach-lookup (global-name "com.apple.system.logger"))`
	args := append([]string{"-p", profile, executable}, meta.Runtime.Command[1:]...)
	return exec.Command("/usr/bin/sandbox-exec", args...), nil // #nosec G204 -- fixed OS sandbox executable with an escaped deny-by-default policy.
}
