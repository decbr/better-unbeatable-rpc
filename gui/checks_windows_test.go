package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

var platformCheckKeys []string

// setUpLoader has nothing to do: Windows loads winhttp.dll from the game folder.
func setUpLoader(*testing.T, string) {}

// writeInstallState writes what install.ps1 would, under the isolated state dir.
func writeInstallState(t *testing.T, state, game string) {
	data, _ := json.Marshal(installState{GameDir: game})
	write(t, filepath.Join(state, "unbeatable-rpc", "install.json"), string(data))
}

func TestDaemonCheck(t *testing.T) {
	for _, tc := range []struct {
		running bool
		svc     ServiceInfo
		level   string
		detail  string
	}{
		{true, ServiceInfo{Installed: true, Enabled: true}, levelOK, "Running 0.3.0 in the background, starting at login."},
		{true, ServiceInfo{Installed: true}, levelOK, "Running 0.3.0 in the background."},
		{true, ServiceInfo{}, levelOK, "Running 0.3.0."},
		{false, ServiceInfo{Installed: true}, levelBad, "Not running."},
		{false, ServiceInfo{}, levelUnknown, "Not running, and not installed next to this app."},
	} {
		c := daemonCheck(tc.running, "0.3.0", tc.svc)
		if c.Level != tc.level || c.Detail != tc.detail {
			t.Errorf("%v %+v: got %s %q", tc.running, tc.svc, c.Level, c.Detail)
		}
	}
}
