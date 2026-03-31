//go:build linux

package hostservice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func servicePath(projectDir string) (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}
	name := ServiceName(projectDir) + ".service"
	return filepath.Join(home, ".config", "systemd", "user", name), "user", "linux", nil
}

func install(projectDir, executable string) (*Metadata, error) {
	meta, err := metadataForProject(projectDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(meta.ServicePath), 0o755); err != nil {
		return nil, err
	}
	meta.Executable = executable
	body := renderSystemdUnit(*meta)
	if err := os.WriteFile(meta.ServicePath, []byte(body), 0o644); err != nil {
		return nil, err
	}
	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		return nil, err
	}
	if err := runCmd("systemctl", "--user", "enable", meta.Name+".service"); err != nil {
		return nil, err
	}
	if err := runCmd("systemctl", "--user", "restart", meta.Name+".service"); err != nil {
		return nil, err
	}
	return meta, nil
}

func uninstall(meta *Metadata) error {
	_ = runCmd("systemctl", "--user", "disable", "--now", meta.Name+".service")
	_ = runCmd("systemctl", "--user", "daemon-reload")
	if err := os.Remove(meta.ServicePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func start(meta *Metadata) error {
	return runCmd("systemctl", "--user", "start", meta.Name+".service")
}

func stop(meta *Metadata) error {
	return runCmd("systemctl", "--user", "stop", meta.Name+".service")
}

func restart(meta *Metadata) error {
	return runCmd("systemctl", "--user", "restart", meta.Name+".service")
}

func status(projectDir string, meta *Metadata) (*Status, error) {
	result := &Status{Metadata: meta}
	if meta == nil {
		return result, nil
	}
	if _, err := os.Stat(meta.ServicePath); err == nil {
		result.Installed = true
	}
	result.Running = cmdSucceeds("systemctl", "--user", "is-active", "--quiet", meta.Name+".service")
	result.Enabled = cmdSucceeds("systemctl", "--user", "is-enabled", "--quiet", meta.Name+".service")
	if !result.Installed {
		result.Message = "service file not installed"
	} else if result.Running {
		result.Message = "service is running"
	} else {
		result.Message = "service is installed but not running"
	}
	return result, nil
}

func renderSystemdUnit(meta Metadata) string {
	return fmt.Sprintf(`[Unit]
Description=Foundry CMS (%s)
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s serve
Restart=always
RestartSec=3
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, escapeSystemd(meta.Name), escapeSystemd(meta.ProjectDir), escapeSystemd(meta.Executable), escapeSystemd(meta.LogPath), escapeSystemd(meta.LogPath))
}

func escapeSystemd(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", " ")
	return replacer.Replace(value)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return nil
}

func cmdSucceeds(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	return cmd.Run() == nil
}
