package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Under Proton, BepInEx's winhttp.dll only loads with a DLL override, kept in
// the game's prefix or set by the launch options.
const (
	appID           = "2240620"
	launchOptions   = `WINEDLLOVERRIDES="winhttp=n,b" %command%`
	overrideSection = `[Software\Wine\DllOverrides]`
)

// platformState is what the Linux checks keep between polls.
type platformState struct {
	regCache regCache
}

func (a *App) platformChecks(game string) []Check {
	return []Check{a.loaderCheck(game)}
}

func (a *App) loaderCheck(game string) Check {
	c := Check{Key: "loader", Label: "BepInEx loading"}
	reg := prefixRegistry(game)
	value, err := a.platform.regCache.winhttpOverride(reg)
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
