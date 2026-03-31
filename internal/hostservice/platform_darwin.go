//go:build darwin

package hostservice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func servicePath(projectDir string) (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}
	name := ServiceLabel(projectDir) + ".plist"
	return filepath.Join(home, "Library", "LaunchAgents", name), "user", "darwin", nil
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
	body := renderLaunchdPlist(*meta)
	if err := os.WriteFile(meta.ServicePath, []byte(body), 0o644); err != nil {
		return nil, err
	}
	_ = runCmd("launchctl", "bootout", launchDomain(), meta.ServicePath)
	if err := runCmd("launchctl", "bootstrap", launchDomain(), meta.ServicePath); err != nil {
		return nil, err
	}
	if err := runCmd("launchctl", "enable", launchDomain()+"/"+meta.Label); err != nil {
		return nil, err
	}
	if err := runCmd("launchctl", "kickstart", "-k", launchDomain()+"/"+meta.Label); err != nil {
		return nil, err
	}
	return meta, nil
}

func uninstall(meta *Metadata) error {
	_ = runCmd("launchctl", "bootout", launchDomain(), meta.ServicePath)
	if err := os.Remove(meta.ServicePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func start(meta *Metadata) error {
	return runCmd("launchctl", "kickstart", "-k", launchDomain()+"/"+meta.Label)
}

func stop(meta *Metadata) error {
	return runCmd("launchctl", "bootout", launchDomain(), meta.ServicePath)
}

func restart(meta *Metadata) error {
	return runCmd("launchctl", "kickstart", "-k", launchDomain()+"/"+meta.Label)
}

func status(projectDir string, meta *Metadata) (*Status, error) {
	result := &Status{Metadata: meta}
	if meta == nil {
		return result, nil
	}
	if _, err := os.Stat(meta.ServicePath); err == nil {
		result.Installed = true
	}
	cmd := exec.Command("launchctl", "print", launchDomain()+"/"+meta.Label)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	err := cmd.Run()
	if err == nil {
		result.Running = true
		result.Enabled = true
		result.Message = "service is running"
		return result, nil
	}
	if result.Installed {
		result.Message = "service is installed but not running"
	} else {
		result.Message = "service file not installed"
	}
	return result, nil
}

func renderLaunchdPlist(meta Metadata) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>serve</string>
  </array>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(meta.Label), xmlEscape(meta.Executable), xmlEscape(meta.ProjectDir), xmlEscape(meta.LogPath), xmlEscape(meta.LogPath))
}

func launchDomain() string {
	current, err := user.Current()
	if err != nil {
		return "gui/0"
	}
	return "gui/" + current.Uid
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
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
