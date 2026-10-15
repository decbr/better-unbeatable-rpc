package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInstalledGameDirReadsInstallerState(t *testing.T) {
	state := isolateState(t)
	if got := installedGameDir(); got != "" {
		t.Errorf("no state file: got %q, want empty", got)
	}
	game := `C:\Program Files (x86)\Steam\steamapps\common\UNBEATABLE`
	write(t, filepath.Join(state, "unbeatable-rpc", "install.json"),
		"\xef\xbb\xbf{\"gameDir\": \"C:\\\\Program Files (x86)\\\\Steam\\\\steamapps\\\\common\\\\UNBEATABLE\", \"bepinexInstalled\": true}")
	if got := installedGameDir(); got != game {
		t.Errorf("got %q, want %q", got, game)
	}
}

func TestProcessesFindsItself(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if pids := processes(filepath.Base(self)); !slices.Contains(pids, uint32(os.Getpid())) {
		t.Errorf("processes(%s) = %v, missing %d", filepath.Base(self), pids, os.Getpid())
	}
	if pids := processes("no-such-process.exe"); len(pids) != 0 {
		t.Errorf("found %v for a process that doesn't exist", pids)
	}
}

func TestDaemonLogTail(t *testing.T) {
	state := isolateState(t)
	write(t, filepath.Join(state, "unbeatable-rpc", "daemon.log"), "a\r\nb\r\nc\r\n")
	if got := daemonLog(2); got != "b\nc" {
		t.Errorf("got %q", got)
	}
}
