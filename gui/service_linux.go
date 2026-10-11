package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const unit = "unbeatable-rpc"

func systemctl(args ...string) error {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func startDaemon() error   { return systemctl("start", unit) }
func restartDaemon() error { return systemctl("restart", unit) }

func setStartAtLogin(on bool) error {
	if on {
		return systemctl("enable", unit)
	}
	return systemctl("disable", unit)
}

// ServiceInfo is the daemon's systemd user service. Installed is false for
// installs that use the launch wrapper instead.
type ServiceInfo struct {
	Installed bool `json:"installed"`
	Active    bool `json:"active"`
	Enabled   bool `json:"enabled"`
}

func serviceInfo() ServiceInfo {
	out, err := exec.Command("systemctl", "--user", "show", unit, "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState").Output()
	if err != nil {
		return ServiceInfo{}
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[k] = v
		}
	}
	return ServiceInfo{
		Installed: props["LoadState"] == "loaded",
		Active:    props["ActiveState"] == "active",
		Enabled:   props["UnitFileState"] == "enabled",
	}
}

// the process name is 14 characters, under the kernel's 15-character comm limit
func gameProcessRunning() bool {
	return exec.Command("pgrep", "-x", "UNBEATABLE.exe").Run() == nil
}

// daemonLog is the daemon's last n log lines: the journal for the service,
// or the launch wrapper's log file.
func daemonLog(n int) string {
	out, err := exec.Command("journalctl", "--user", "-u", unit, "-n", strconv.Itoa(n), "--no-pager", "-o", "short-iso").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 && !strings.Contains(string(out), "-- No entries --") {
		return strings.TrimSpace(string(out))
	}
	state := filepath.Join(os.Getenv("HOME"), ".local", "state")
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		state = dir
	}
	data, err := os.ReadFile(filepath.Join(state, "unbeatable-rpc", "daemon.log"))
	if err != nil {
		return ""
	}
	return lastLines(string(data), n)
}

// uninstallerPath is the copy of install.sh the installer puts next to this app.
func uninstallerPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(self), "unbeatable-rpc-uninstall")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("couldn't find %s; run ./uninstall.sh from the release folder instead", path)
	}
	return path, nil
}

func runUninstaller(path string, all bool) (string, error) {
	var args []string
	if all {
		args = append(args, "--all")
	}
	out, err := exec.Command(path, args...).CombinedOutput()
	return string(out), err
}

// installedGameDir is the game folder the installer used, from its state file.
// bash sources it, as the installer does, since it's written with printf %q.
func installedGameDir() string {
	state := filepath.Join(os.Getenv("HOME"), ".local", "state", "unbeatable-rpc", "install.env")
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		state = filepath.Join(dir, "unbeatable-rpc", "install.env")
	}
	if _, err := os.Stat(state); err != nil {
		return ""
	}
	out, err := exec.Command("bash", "-c", `source "$1" && printf %s "$GAME_DIR"`, "_", state).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func openFolder(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't open %s: %w", path, err)
	}
	go cmd.Wait()
	return nil
}
