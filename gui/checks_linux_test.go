package main

import (
	"os"
	"path/filepath"
	"testing"
)

// the checks only Linux has, for TestHealthFullInstall
var platformCheckKeys = []string{"loader"}

// setUpLoader makes the Proton prefix load winhttp.dll natively.
func setUpLoader(t *testing.T, game string) {
	write(t, prefixRegistry(game), "[Software\\\\Wine\\\\DllOverrides] 1\n\"winhttp\"=\"native,builtin\"\n\n")
}

// writeInstallState writes what install.sh would, under the isolated state dir.
func writeInstallState(t *testing.T, state, game string) {
	write(t, filepath.Join(state, "unbeatable-rpc", "install.env"), "GAME_DIR="+game+"\n")
}

func TestPrefixRegistry(t *testing.T) {
	got := prefixRegistry("/lib/steamapps/common/UNBEATABLE")
	if want := "/lib/steamapps/compatdata/2240620/pfx/user.reg"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestWinhttpOverride(t *testing.T) {
	dir := t.TempDir()
	set := filepath.Join(dir, "set.reg")
	write(t, set, "WINE REGISTRY Version 2\n\n[Software\\\\Wine\\\\Drivers] 1\n\"winhttp\"=\"wrong section\"\n\n"+
		"[Software\\\\Wine\\\\DllOverrides] 1712345678\n#time=1\n\"d3d11\"=\"native\"\n\"winhttp\"=\"native,builtin\"\n\n[Other] 2\n")
	if v, err := winhttpOverride(set); err != nil || v != "native,builtin" {
		t.Errorf("set: %q, %v", v, err)
	}
	unset := filepath.Join(dir, "unset.reg")
	write(t, unset, "[Software\\\\Wine\\\\DllOverrides] 1\n\"d3d11\"=\"native\"\n\n[Next] 2\n\"winhttp\"=\"native\"\n")
	if v, err := winhttpOverride(unset); err != nil || v != "" {
		t.Errorf("unset: %q, %v", v, err)
	}
	if _, err := winhttpOverride(filepath.Join(dir, "missing.reg")); !os.IsNotExist(err) {
		t.Errorf("missing file: %v", err)
	}

	var c regCache
	if v, _ := c.winhttpOverride(set); v != "native,builtin" {
		t.Errorf("cache miss: %q", v)
	}
	write(t, set, "[Software\\\\Wine\\\\DllOverrides] 1\n\"winhttp\"=\"builtin\"\n")
	os.Chtimes(set, timeNowPlus(t, 10), timeNowPlus(t, 10))
	if v, _ := c.winhttpOverride(set); v != "builtin" {
		t.Errorf("cache didn't notice the change: %q", v)
	}
}

func TestDaemonCheck(t *testing.T) {
	for _, tc := range []struct {
		running bool
		svc     ServiceInfo
		level   string
		detail  string
	}{
		{true, ServiceInfo{Installed: true, Enabled: true}, levelOK, "Running 0.3.0, as a systemd service that starts at login."},
		{true, ServiceInfo{Installed: true}, levelOK, "Running 0.3.0, as a systemd service."},
		{true, ServiceInfo{}, levelOK, "Running 0.3.0 (launch wrapper)."},
		{false, ServiceInfo{Installed: true}, levelBad, "Not running."},
		{false, ServiceInfo{}, levelUnknown, "Not running. With the launch wrapper it starts with the game."},
	} {
		c := daemonCheck(tc.running, "0.3.0", tc.svc)
		if c.Level != tc.level || c.Detail != tc.detail {
			t.Errorf("%v %+v: got %s %q", tc.running, tc.svc, c.Level, c.Detail)
		}
	}
}
