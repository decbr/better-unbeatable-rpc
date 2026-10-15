package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/status"
)

const (
	levelOK      = "ok"
	levelWarn    = "warn"
	levelBad     = "bad"
	levelUnknown = "unknown"
)

type Check struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

type Health struct {
	Checks        []Check `json:"checks"`
	PluginVersion string  `json:"pluginVersion,omitempty"` // from the last launch's log
	GameOpen      bool    `json:"gameOpen"`                // the game process is running
}

func (a *App) Health() Health {
	game := installedGameDir()
	h := Health{GameOpen: gameProcessRunning()}
	daemon, daemonErr := status.Fetch(a.ctx, a.statusPort)
	h.Checks = []Check{daemonCheck(daemonErr == nil, daemon.Version, serviceInfo())}
	if game == "" {
		h.Checks = append(h.Checks, Check{Key: "game", Label: "Game folder", Level: levelUnknown,
			Detail: "Not known: the mod wasn't installed with " + installerName + ", so these checks can't run.",
			Fix:    "Re-run " + installerName + " from the release folder."})
		return h
	}

	h.Checks = append(h.Checks, fileCheck("bepinex", "BepInEx", "Installed", "Not installed in the game folder.",
		filepath.Join(game, "BepInEx", "core", "BepInEx.dll"), filepath.Join(game, "winhttp.dll")))
	h.Checks = append(h.Checks, fileCheck("plugin", "Mod", "Installed", "Not in BepInEx/plugins.",
		filepath.Join(game, "BepInEx", "plugins", "UnbeatableRPC", "UnbeatableRPC.dll")))
	h.Checks = append(h.Checks, a.platformChecks(game)...)

	log := readBepInExLog(filepath.Join(game, "BepInEx", "LogOutput.log"))
	h.PluginVersion = log.Version
	h.Checks = append(h.Checks, loadedCheck(log))

	if log.Version != "" && daemonErr == nil && daemon.Version != "dev" {
		c := Check{Key: "versions", Label: "Versions", Level: levelOK, Detail: "Mod and daemon are both " + log.Version}
		if log.Version != daemon.Version {
			c.Level = levelWarn
			c.Detail = fmt.Sprintf("Mod %s (last launch), daemon %s.", log.Version, daemon.Version)
			c.Fix = "Re-run the installer so they match."
		}
		h.Checks = append(h.Checks, c)
	}

	if h.GameOpen && daemonErr == nil {
		c := Check{Key: "sending", Label: "Game", Level: levelOK, Detail: "Open, and the mod is talking to the daemon."}
		if !daemon.GameRunning {
			c.Level = levelBad
			c.Detail = "Open, but the mod isn't sending anything."
			c.Fix = "If the checks above are fine, look at BepInEx's LogOutput.log (Files → BepInEx)."
		}
		h.Checks = append(h.Checks, c)
	}
	return h
}

func daemonCheck(running bool, version string, svc ServiceInfo) Check {
	c := Check{Key: "daemon", Label: "Daemon"}
	switch {
	case running && svc.Installed:
		c.Level, c.Detail = levelOK, "Running "+version+serviceText
		if svc.Enabled {
			c.Detail += startsAtLoginText
		}
		c.Detail += "."
	case running:
		c.Level, c.Detail = levelOK, "Running "+version+noServiceText
	case svc.Installed:
		c.Level, c.Detail, c.Fix = levelBad, "Not running.", "Start or restart it on the Daemon tab."
	default:
		c.Level, c.Detail = levelUnknown, noServiceStoppedText
	}
	return c
}

func fileCheck(key, label, ok, missing string, paths ...string) Check {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return Check{Key: key, Label: label, Level: levelBad, Detail: missing, Fix: "Re-run the installer."}
		}
	}
	return Check{Key: key, Label: label, Level: levelOK, Detail: ok}
}

func loadedCheck(log bepinexLog) Check {
	c := Check{Key: "loaded", Label: "Last launch"}
	switch {
	case !log.Exists:
		c.Level = levelUnknown
		c.Detail = "BepInEx hasn't run yet."
		c.Fix = "Launch the game once."
	case !log.Loaded:
		c.Level = levelBad
		c.Detail = "BepInEx ran " + ago(log.Modified) + " but didn't load the mod."
		c.Fix = "Re-run the installer."
	case len(log.FailedPatches) > 0:
		c.Level = levelWarn
		c.Detail = fmt.Sprintf("Mod %s loaded %s, but these didn't apply: %s.", log.Version, ago(log.Modified), strings.Join(log.FailedPatches, ", "))
		c.Fix = "A game update probably changed them; check for a mod update."
	default:
		c.Level = levelOK
		c.Detail = fmt.Sprintf("Mod %s loaded %s.", log.Version, ago(log.Modified))
	}
	return c
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return t.Format("2 Jan")
	}
}

type bepinexLog struct {
	Exists        bool
	Loaded        bool
	Version       string
	FailedPatches []string
	Modified      time.Time
}

var (
	loadingRe = regexp.MustCompile(`Loading \[UnbeatableRPC ([^\]]+)\]`)
	failedRe  = regexp.MustCompile(`Failed to apply (\w+)`)
)

func readBepInExLog(path string) bepinexLog {
	var l bepinexLog
	fi, err := os.Stat(path)
	if err != nil {
		return l
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return l
	}
	l.Exists, l.Modified = true, fi.ModTime()
	for _, line := range strings.Split(string(data), "\n") {
		if m := loadingRe.FindStringSubmatch(line); m != nil {
			l.Loaded, l.Version = true, m[1]
		}
		if m := failedRe.FindStringSubmatch(line); m != nil {
			l.FailedPatches = append(l.FailedPatches, m[1])
		}
	}
	return l
}
