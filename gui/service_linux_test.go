package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunUninstaller(t *testing.T) {
	script := filepath.Join(t.TempDir(), "unbeatable-rpc-uninstall")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"args: $*\"\n[ \"$1\" != fail ] || { echo 'install.sh: UNBEATABLE is running; quit it first'; exit 1; }\n"), 0o755)

	if out, err := runUninstaller(script, false); err != nil || out != "args: \n" {
		t.Errorf("plain: %q, %v", out, err)
	}
	if out, err := runUninstaller(script, true); err != nil || out != "args: --all\n" {
		t.Errorf("--all: %q, %v", out, err)
	}
	out, err := runUninstaller(script+"-missing", false)
	if err == nil {
		t.Errorf("missing uninstaller ran: %q", out)
	}
	if got := lastLines("a\nb\nc\nd\n", 2); got != "c\nd" {
		t.Errorf("lastLines = %q", got)
	}
}

func TestInstalledGameDirReadsInstallerState(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if got := installedGameDir(); got != "" {
		t.Errorf("no state file: got %q, want empty", got)
	}
	// the installer writes it with printf %q, which escapes spaces and quotes
	game := filepath.Join(t.TempDir(), "Steam Library", "it's UNBEATABLE")
	os.MkdirAll(filepath.Join(state, "unbeatable-rpc"), 0o755)
	script := `printf 'GAME_DIR=%q\nSERVICE=%q\n' "$1" true >"$2"`
	if err := exec.Command("bash", "-c", script, "_", game, filepath.Join(state, "unbeatable-rpc", "install.env")).Run(); err != nil {
		t.Fatal(err)
	}
	if got := installedGameDir(); got != game {
		t.Errorf("got %q, want %q", got, game)
	}
}
