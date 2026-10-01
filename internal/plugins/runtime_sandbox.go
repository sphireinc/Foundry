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
	if len(meta.Runtime.Command) == 0 || strings.TrimSpace(meta.Runtime.Command[0]) == "" {
		return "", "", fmt.Errorf("strict RPC command requires an executable")
	}
	root, err := filepath.Abs(meta.Directory)
	if err != nil {
		return "", "", err
	}
	artifactRoot := root
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	rel := meta.Runtime.Command[0]
	if filepath.IsAbs(rel) {
		rel, err = filepath.Rel(root, rel)
		// Artifact roots can themselves have a canonical alias (for example,
		// macOS /var -> /private/var). Accept absolute paths under either name.
		if err == nil && !filepath.IsLocal(rel) {
			rel, err = filepath.Rel(artifactRoot, meta.Runtime.Command[0])
		}
		if err != nil {
			return "", "", err
		}
	}
	if !filepath.IsLocal(rel) {
		return "", "", fmt.Errorf("strict RPC executable must be inside the plugin artifact directory")
	}
	// Open through the artifact root: symlinks cannot escape it between path
	// validation and the executable metadata lookup.
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = scoped.Close() }()
	file, err := scoped.Open(rel)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", "", fmt.Errorf("strict RPC command must be a prebuilt executable")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		return "", "", err
	}
	if !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", "", fmt.Errorf("strict RPC executable must be inside the plugin artifact directory")
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
