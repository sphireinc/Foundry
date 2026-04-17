package standalone

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	RunDirName   = ".foundry/run"
	StateFile    = "standalone.json"
	LogFile      = "standalone.log"
	ManagedBin   = "foundry-standalone"
	defaultLines = 120
)

var buildStandaloneBinary = func(projectDir, target string) error {
	cmd := exec.Command("go", "build", "-o", target, "./cmd/foundry")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+runtime.GOOS,
		"GOARCH="+runtime.GOARCH,
	)
	return cmd.Run()
}

type State struct {
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	ProjectDir string    `json:"project_dir"`
	LogPath    string    `json:"log_path"`
	Command    []string  `json:"command"`
}

type Paths struct {
	RunDir    string
	StatePath string
	LogPath   string
}

func ProjectPaths(projectDir string) Paths {
	runDir := filepath.Join(projectDir, RunDirName)
	return Paths{
		RunDir:    runDir,
		StatePath: filepath.Join(runDir, StateFile),
		LogPath:   filepath.Join(runDir, LogFile),
	}
}

func EnsureRunDir(projectDir string) (Paths, error) {
	paths := ProjectPaths(projectDir)
	if err := os.MkdirAll(paths.RunDir, 0o755); err != nil {
		return Paths{}, err
	}
	return paths, nil
}

func LoadState(projectDir string) (*State, error) {
	paths := ProjectPaths(projectDir)
	body, err := os.ReadFile(paths.StatePath)
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func SaveState(projectDir string, state State) error {
	paths, err := EnsureRunDir(projectDir)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.StatePath, body, 0o644)
}

func RemoveState(projectDir string) error {
	paths := ProjectPaths(projectDir)
	if err := os.Remove(paths.StatePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RunningState(projectDir string) (*State, bool, error) {
	state, err := LoadState(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if state.PID > 0 && IsProcessAlive(state.PID) {
		return state, true, nil
	}
	return state, false, nil
}

func Start(projectDir string, rawArgs []string) (*State, error) {
	if _, running, err := RunningState(projectDir); err != nil {
		return nil, err
	} else if running {
		return nil, fmt.Errorf("Foundry standalone server is already running")
	}

	command, err := LaunchCommand(projectDir, rawArgs)
	if err != nil {
		return nil, err
	}
	return startWithCommand(projectDir, command)
}

func startWithCommand(projectDir string, command []string) (state *State, retErr error) {
	if _, running, err := RunningState(projectDir); err != nil {
		return nil, err
	} else if running {
		return nil, fmt.Errorf("Foundry standalone server is already running")
	}

	paths, err := EnsureRunDir(projectDir)
	if err != nil {
		return nil, err
	}
	_ = RemoveState(projectDir)
	if len(command) == 0 {
		return nil, fmt.Errorf("could not construct standalone launch command")
	}

	logFile, err := os.OpenFile(paths.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := logFile.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = projectDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	devNull, err := os.Open(os.DevNull)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
	}
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	state = &State{
		PID:        cmd.Process.Pid,
		StartedAt:  time.Now().UTC(),
		ProjectDir: projectDir,
		LogPath:    paths.LogPath,
		Command:    append([]string(nil), command...),
	}
	if err := SaveState(projectDir, *state); err != nil {
		return nil, err
	}
	return state, nil
}

func Stop(projectDir string) error {
	state, running, err := RunningState(projectDir)
	if err != nil {
		return err
	}
	if state == nil || !running {
		_ = RemoveState(projectDir)
		return fmt.Errorf("Foundry standalone server is not running")
	}

	if err := sendTerminate(state.PID); err != nil {
		return err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !IsProcessAlive(state.PID) {
			_ = RemoveState(projectDir)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := sendKill(state.PID); err != nil {
		return err
	}
	_ = RemoveState(projectDir)
	return nil
}

func Restart(projectDir string, rawArgs []string) (*State, error) {
	state, running, err := RunningState(projectDir)
	if err != nil {
		return nil, err
	}
	command := []string(nil)
	if state != nil && len(state.Command) > 0 {
		command = append([]string(nil), state.Command...)
	}
	if state != nil && running {
		if err := Stop(projectDir); err != nil {
			return nil, err
		}
	}
	if len(command) > 0 {
		return startWithCommand(projectDir, command)
	}
	return Start(projectDir, rawArgs)
}

func LaunchCommand(projectDir string, rawArgs []string) ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	if shouldUseGoRun(exe, projectDir) {
		managedBinary, err := ensureManagedBinary(projectDir)
		if err != nil {
			return nil, err
		}
		raw := append([]string(nil), rawArgs[1:]...)
		replaced := false
		for i, arg := range raw {
			if strings.TrimSpace(arg) == "serve-standalone" {
				raw[i] = "serve"
				replaced = true
				break
			}
		}
		if !replaced {
			raw = append([]string{"serve"}, raw...)
		}
		return append([]string{managedBinary}, raw...), nil
	}

	args := append([]string(nil), rawArgs[1:]...)
	replaced := false
	for i, arg := range args {
		if strings.TrimSpace(arg) == "serve-standalone" {
			args[i] = "serve"
			replaced = true
			break
		}
	}
	if !replaced {
		args = []string{"serve"}
	}
	return append([]string{exe}, args...), nil
}

func ensureManagedBinary(projectDir string) (string, error) {
	paths, err := EnsureRunDir(projectDir)
	if err != nil {
		return "", err
	}
	name := ManagedBin
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(paths.RunDir, name)
	if _, err := exec.LookPath("go"); err != nil {
		return "", fmt.Errorf("foundry was launched via go run but go is not available in PATH")
	}
	if err := buildStandaloneBinary(projectDir, target); err != nil {
		return "", fmt.Errorf("build managed standalone binary: %w", err)
	}
	return target, nil
}
func shouldUseGoRun(executablePath, projectDir string) bool {
	exe := filepath.Clean(executablePath)
	tmp := filepath.Clean(os.TempDir())
	if strings.Contains(exe, string(filepath.Separator)+"go-build"+string(filepath.Separator)) {
		return fileExists(filepath.Join(projectDir, "cmd", "foundry", "main.go"))
	}
	if strings.HasPrefix(exe, tmp+string(filepath.Separator)) && fileExists(filepath.Join(projectDir, "cmd", "foundry", "main.go")) {
		return true
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ReadLastLines(path string, lines int) (string, error) {
	if lines <= 0 {
		lines = defaultLines
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	all := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if len(all) > 0 && all[len(all)-1] == "" {
		all = all[:len(all)-1]
	}
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}

func FollowLog(path string, out io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if _, werr := io.WriteString(out, line); werr != nil {
				return werr
			}
		}
		if err == nil {
			continue
		}
		if err != io.EOF {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}
