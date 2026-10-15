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
	isolateState(t)
	h := a.Health()
	if len(h.Checks) != 2 || h.Checks[0].Key != "daemon" || h.Checks[1].Level != levelUnknown {
		t.Errorf("got %+v, want the daemon check then one unknown check", h.Checks)
	}
}

func TestHealthFullInstall(t *testing.T) {
	a := newTestApp(t)
	state := isolateState(t)
	lib := t.TempDir()
	game := filepath.Join(lib, "steamapps", "common", "UNBEATABLE")
	writeInstallState(t, state, game)
	write(t, filepath.Join(game, "winhttp.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "core", "BepInEx.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "plugins", "UnbeatableRPC", "UnbeatableRPC.dll"), "")
	write(t, filepath.Join(game, "BepInEx", "LogOutput.log"), "Loading [UnbeatableRPC 0.1.0]\n")
	setUpLoader(t, game)

	h := a.Health()
	levels := map[string]string{}
	for _, c := range h.Checks {
		levels[c.Key] = c.Level
	}
	for _, key := range append([]string{"bepinex", "plugin", "loaded"}, platformCheckKeys...) {
		if levels[key] != levelOK {
			t.Errorf("%s = %q, want ok (all: %+v)", key, levels[key], h.Checks)
		}
	}
	if h.PluginVersion != "0.1.0" {
		t.Errorf("plugin version %q", h.PluginVersion)
	}
}
