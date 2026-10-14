package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/status"
)

const (
	levelOK      = "ok"
	levelWarn    = "warn"
	levelBad     = "bad"
	levelUnknown = "unknown"

	appID           = "2240620"
	launchOptions   = `WINEDLLOVERRIDES="winhttp=n,b" %command%`
	overrideSection = `[Software\\Wine\\DllOverrides]`
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
			Detail: "Not known: the mod wasn't installed with install.sh, so these checks can't run.",
			Fix:    "Re-run ./install.sh from the release folder."})
		return h
	}

	h.Checks = append(h.Checks, fileCheck("bepinex", "BepInEx", "Installed", "Not installed in the game folder.",
		filepath.Join(game, "BepInEx", "core", "BepInEx.dll"), filepath.Join(game, "winhttp.dll")))
	h.Checks = append(h.Checks, fileCheck("plugin", "Mod", "Installed", "Not in BepInEx/plugins.",
		filepath.Join(game, "BepInEx", "plugins", "UnbeatableRPC", "UnbeatableRPC.dll")))
	h.Checks = append(h.Checks, a.loaderCheck(game))

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
		c.Level, c.Detail = levelOK, "Running "+version+", as a systemd service"
		if svc.Enabled {
			c.Detail += " that starts at login"
		}
		c.Detail += "."
	case running:
		c.Level, c.Detail = levelOK, "Running "+version+" (launch wrapper)."
	case svc.Installed:
		c.Level, c.Detail, c.Fix = levelBad, "Not running.", "Start or restart it on the Daemon tab."
	default:
		c.Level, c.Detail = levelUnknown, "Not running. With the launch wrapper it starts with the game."
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

func (a *App) loaderCheck(game string) Check {
	c := Check{Key: "loader", Label: "BepInEx loading"}
	reg := prefixRegistry(game)
	value, err := a.regCache.winhttpOverride(reg)
	switch {
	case os.IsNotExist(err):
		c.Level = levelUnknown
		c.Detail = "The game's Proton prefix doesn't exist yet."
		c.Fix = "Launch the game once, then re-run the installer, or set the launch options to " + launchOptions
	case err != nil:
		c.Level = levelUnknown
		c.Detail = "Couldn't read the Proton prefix: " + err.Error()
	case strings.HasPrefix(value, "native"):
		c.Level = levelOK
		c.Detail = "Set up in the Proton prefix; no launch options needed."
	default:
		c.Level = levelWarn
		c.Detail = "Not set up in the Proton prefix."
		c.Fix = "Set the game's launch options to " + launchOptions + ", or re-run the installer to store it in the prefix."
	}
	return c
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

// prefixRegistry is the Proton prefix's user.reg, as install.sh finds it:
// <library>/steamapps/common/UNBEATABLE -> <library>/steamapps/compatdata/<app>/pfx.
func prefixRegistry(game string) string {
	lib := filepath.Clean(filepath.Join(game, "..", "..", ".."))
	return filepath.Join(lib, "steamapps", "compatdata", appID, "pfx", "user.reg")
}

// regCache remembers the winhttp override per user.reg until the file
// changes: the file is megabytes and the page polls every 2 s.
type regCache struct {
	mu    sync.Mutex
	path  string
	stamp time.Time
	size  int64
	value string
}

func (c *regCache) winhttpOverride(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == path && c.stamp.Equal(fi.ModTime()) && c.size == fi.Size() {
		return c.value, nil
	}
	value, err := winhttpOverride(path)
	if err != nil {
		return "", err
	}
	c.path, c.stamp, c.size, c.value = path, fi.ModTime(), fi.Size(), value
	return value, nil
}

// winhttpOverride reads "winhttp" from the DllOverrides section, like
// install.sh's registry_override. Empty if it isn't set.
func winhttpOverride(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	inside := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, overrideSection):
			inside = true
		case inside && line == "":
			return "", nil
		case inside && strings.HasPrefix(line, `"winhttp"="`):
			return strings.TrimSuffix(strings.TrimPrefix(line, `"winhttp"="`), `"`), nil
		}
	}
	return "", sc.Err()
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
