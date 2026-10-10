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
