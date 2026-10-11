package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/status"
)

var checkMarks = map[string]string{levelOK: "✓", levelWarn: "!", levelBad: "✕", levelUnknown: "?"}

// Diagnostics is the report for a bug report; CopyText puts it on the
// clipboard, and the page shows it for copying by hand if that fails.
func (a *App) Diagnostics() string { return a.diagnostics() }

func (a *App) CopyText(text string) error { return copyText(a.ctx, text) }

func (a *App) diagnostics() string {
	var b strings.Builder
	section := func(title string) { fmt.Fprintf(&b, "\n== %s\n", title) }

	h := a.Health()
	daemon, daemonErr := status.Fetch(a.ctx, a.statusPort)
	daemonVersion := "not running"
	if daemonErr == nil {
		daemonVersion = daemon.Version
	}
	pluginVersion := h.PluginVersion
	if pluginVersion == "" {
		pluginVersion = "unknown"
	}

	fmt.Fprintf(&b, "UNBEATABLE RPC diagnostics, %s\n", time.Now().Format("2006-01-02 15:04 MST"))
	fmt.Fprintf(&b, "App %s, daemon %s, mod %s (last launch)\n", version, daemonVersion, pluginVersion)
	fmt.Fprintf(&b, "%s/%s, desktop %s, session %s, game open: %v\n", goruntime.GOOS, goruntime.GOARCH,
		orNone(os.Getenv("XDG_CURRENT_DESKTOP")), orNone(os.Getenv("XDG_SESSION_TYPE")), h.GameOpen)

	section("Checks")
	for _, c := range h.Checks {
		fmt.Fprintf(&b, "%s %s: %s\n", checkMarks[c.Level], c.Label, c.Detail)
	}

	section("Daemon status")
	if daemonErr != nil {
		fmt.Fprintf(&b, "unreachable: %v\n", daemonErr)
	} else if j, err := json.MarshalIndent(daemon, "", "  "); err == nil {
		b.Write(j)
		b.WriteByte('\n')
	}

	section("Settings file")
	path := a.configPath()
	if data, err := os.ReadFile(path); err != nil {
		fmt.Fprintf(&b, "%s: %v\n", path, err)
	} else {
		fmt.Fprintf(&b, "%s\n%s\n", path, strings.TrimSpace(string(data)))
	}

	section("Mod settings")
	if p := pluginConfigPath(); p == "" {
		b.WriteString("game folder unknown\n")
	} else if values, err := readPluginConfig(p); err != nil {
		fmt.Fprintf(&b, "%v\n", err)
	} else {
		for _, k := range []string{"UdpPort", "DisableGameDiscord", "DebugLogging"} {
			fmt.Fprintf(&b, "%s = %s\n", k, orNone(values[k]))
		}
	}

	section("Daemon log (last 30 lines)")
	b.WriteString(orNone(daemonLog(30)) + "\n")

	section("BepInEx log (mod lines, warnings and errors; last 30)")
	if game := installedGameDir(); game == "" {
		b.WriteString("game folder unknown\n")
	} else {
		b.WriteString(orNone(bepinexLogExcerpt(filepath.Join(game, "BepInEx", "LogOutput.log"), 30)) + "\n")
	}
	return b.String()
}

func bepinexLogExcerpt(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	var keep []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		if strings.Contains(line, "UnbeatableRPC") || strings.HasPrefix(line, "[Warning") ||
			strings.HasPrefix(line, "[Error") || strings.HasPrefix(line, "[Fatal") {
			keep = append(keep, line)
		}
	}
	if len(keep) > n {
		keep = keep[len(keep)-n:]
	}
	return strings.Join(keep, "\n")
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}
