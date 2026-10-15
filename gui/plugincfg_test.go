package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleCfg = "## Settings file was created by plugin UnbeatableRPC v0.1.0\r\n\r\n[General]\r\n\r\n" +
	"## UDP port\r\n# Default value: 47820\r\nUdpPort = 47820\r\n\r\n" +
	"## Stop the game's integration\r\n# Default value: true\r\nDisableGameDiscord = true\r\n\r\n" +
	"## Log more\r\nDebugLogging = false\r\n\r\n[State]\r\n\r\nLastKnownStars = 8.95\r\nDebugLogging = keep-me\r\n"

func TestPluginConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), pluginConfigName)
	os.WriteFile(path, []byte(sampleCfg), 0o640)

	values, err := readPluginConfig(path)
	if err != nil || values["DisableGameDiscord"] != "true" || values["DebugLogging"] != "false" || values["UdpPort"] != "47820" {
		t.Fatalf("read %v, %v", values, err)
	}
	if err := setPluginConfig(path, map[string]string{"DisableGameDiscord": "false", "DebugLogging": "true"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := strings.NewReplacer("DisableGameDiscord = true", "DisableGameDiscord = false",
		"DebugLogging = false", "DebugLogging = true").Replace(sampleCfg)
	if string(data) != want {
		t.Errorf("rewrote to:\n%q\nwant:\n%q", data, want)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Errorf("mode changed to %v", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("left %d files behind", len(entries))
	}
}

func TestSetPluginConfigMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), pluginConfigName)
	os.WriteFile(path, []byte("[General]\nUdpPort = 1\n"), 0o644)
	if err := setPluginConfig(path, map[string]string{"DebugLogging": "true"}); err == nil {
		t.Error("missing key accepted")
	}
}
