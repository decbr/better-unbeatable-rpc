package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const pluginConfigName = "xyz.diaduck.unbeatable-rpc.cfg"

type PluginSettings struct {
	Path               string `json:"path"`
	Available          bool   `json:"available"` // the file exists (the game has run with the mod)
	Locked             bool   `json:"locked"`    // the game is open and would overwrite changes
	DisableGameDiscord bool   `json:"disableGameDiscord"`
	DebugLogging       bool   `json:"debugLogging"`
}

func pluginConfigPath() string {
	game := installedGameDir()
	if game == "" {
		return ""
	}
	return filepath.Join(game, "BepInEx", "config", pluginConfigName)
}

func (a *App) PluginSettings() PluginSettings {
	s := PluginSettings{Path: pluginConfigPath(), Locked: gameProcessRunning(), DisableGameDiscord: true}
	if s.Path == "" {
		return s
	}
	values, err := readPluginConfig(s.Path)
	if err != nil {
		return s
	}
	s.Available = true
	if b, err := strconv.ParseBool(values["DisableGameDiscord"]); err == nil {
		s.DisableGameDiscord = b
	}
	if b, err := strconv.ParseBool(values["DebugLogging"]); err == nil {
		s.DebugLogging = b
	}
	return s
}

func (a *App) SavePluginSettings(disableGameDiscord, debugLogging bool) error {
	path := pluginConfigPath()
	if path == "" {
		return errors.New("the game folder isn't known")
	}
	if gameProcessRunning() {
		return errors.New("close the game first: it rewrites this file and would undo the change")
	}
	return setPluginConfig(path, map[string]string{
		"DisableGameDiscord": strconv.FormatBool(disableGameDiscord),
		"DebugLogging":       strconv.FormatBool(debugLogging),
	})
}

// readPluginConfig returns the [General] keys of a BepInEx config file.
func readPluginConfig(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		if section != "[General]" || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return values, nil
}

// setPluginConfig changes the given [General] keys in place, keeping every
// other line (comments, other keys and sections) as it was.
func setPluginConfig(path string, values map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	done := map[string]bool{}
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			section = t
			continue
		}
		if section != "[General]" || strings.HasPrefix(t, "#") {
			continue
		}
		if k, _, ok := strings.Cut(t, "="); ok {
			if v, want := values[strings.TrimSpace(k)]; want {
				// BepInEx writes CRLF under Proton; keep the line's ending
				end := ""
				if strings.HasSuffix(line, "\r") {
					end = "\r"
				}
				lines[i] = strings.TrimSpace(k) + " = " + v + end
				done[strings.TrimSpace(k)] = true
			}
		}
	}
	for k := range values {
		if !done[k] {
			return fmt.Errorf("%s has no %s setting; launch the game once to recreate it", filepath.Base(path), k)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".unbeatable-rpc-*.cfg")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(strings.Join(lines, "\n")); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		os.Chmod(tmp.Name(), fi.Mode())
	}
	return os.Rename(tmp.Name(), path)
}
