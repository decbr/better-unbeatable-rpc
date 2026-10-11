package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestReadBepInExLog(t *testing.T) {
	dir := t.TempDir()
	if l := readBepInExLog(filepath.Join(dir, "none.log")); l.Exists {
		t.Error("missing log reported as existing")
	}
	ok := filepath.Join(dir, "ok.log")
	write(t, ok, "[Message:   BepInEx] Chainloader started\r\n[Info   :   BepInEx] Loading [UnbeatableRPC 0.2.0]\r\n"+
		"[Info   :UnbeatableRPC] Patched DiscordComponentAwake\r\n[Error  :UnbeatableRPC] Failed to apply HighScoreScreenStart: boom\r\n")
	l := readBepInExLog(ok)
	if !l.Exists || !l.Loaded || l.Version != "0.2.0" || strings.Join(l.FailedPatches, ",") != "HighScoreScreenStart" {
		t.Errorf("got %+v", l)
	}
	if c := loadedCheck(l); c.Level != levelWarn || !strings.Contains(c.Detail, "HighScoreScreenStart") {
		t.Errorf("check = %+v", c)
	}
	notLoaded := filepath.Join(dir, "other.log")
	write(t, notLoaded, "[Message:   BepInEx] Chainloader started\n")
	if c := loadedCheck(readBepInExLog(notLoaded)); c.Level != levelBad {
		t.Errorf("not loaded: %+v", c)
	}
}

func TestHealthWithoutInstallerState(t *testing.T) {
	a := newTestApp(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	h := a.Health()
	if len(h.Checks) != 1 || h.Checks[0].Level != levelUnknown {
		t.Errorf("got %+v, want one unknown check", h.Checks)
	}
}

func TestHealthFullInstall(t *testing.T) {
	a := newTestApp(t)
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	lib := t.TempDir()
	game := filepath.Join(lib, "steamapps", "common", "UNBEATABLE")
	write(t, filepath.Join(state, "unbeatable-rpc", "install.env"), "GAME_DIR="+game+"\n")
	write(t, filepath.Join(game, "winhttp.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "core", "BepInEx.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "plugins", "UnbeatableRPC", "UnbeatableRPC.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "LogOutput.log"), "Loading [UnbeatableRPC 0.1.0]\n")
	write(t, prefixRegistry(game), "[Software\\\\Wine\\\\DllOverrides] 1\n\"winhttp\"=\"native,builtin\"\n\n")

	h := a.Health()
	levels := map[string]string{}
	for _, c := range h.Checks {
		levels[c.Key] = c.Level
	}
	for _, key := range []string{"bepinex", "plugin", "loader", "loaded"} {
		if levels[key] != levelOK {
			t.Errorf("%s = %q, want ok (all: %+v)", key, levels[key], h.Checks)
		}
	}
	if h.PluginVersion != "0.1.0" {
		t.Errorf("plugin version %q", h.PluginVersion)
	}
}
