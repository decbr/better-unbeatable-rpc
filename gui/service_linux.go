package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func startDaemon() error {
	out, err := exec.Command("systemctl", "--user", "start", "unbeatable-rpc").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("couldn't start the unbeatable-rpc service: %s", msg)
	}
	return nil
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
