package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
	"github.com/decbr/better-unbeatable-rpc/daemon/status"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend. each exported method is callable from JS as window.go.main.App.<Method>().
type App struct {
	ctx        context.Context
	statusPort int
	regCache   regCache
}

func NewApp() *App {
	port := status.DefaultPort
	if n, err := strconv.Atoi(os.Getenv(status.PortEnv)); err == nil {
		port = n
	}
	return &App{statusPort: port}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

type View struct {
	ConfigPath string            `json:"configPath"`
	File       config.Config     `json:"file"`
	FileError  string            `json:"fileError,omitempty"`
	Defaults   config.Config     `json:"defaults"`
	Styles     []StyleInfo       `json:"styles"`
	EnvVars    map[string]string `json:"envVars"`
}

type StyleInfo struct {
	Name     string    `json:"name"`
	Previews []Preview `json:"previews"`
}

type Preview struct {
	Where   string `json:"where"`
	Details string `json:"details"`
	State   string `json:"state"`
}

func (a *App) Load() View {
	v := View{Defaults: config.Defaults(), EnvVars: config.EnvVars}
	v.ConfigPath = a.configPath()
	file, err := config.Read(v.ConfigPath)
	if err != nil {
		v.FileError = err.Error()
	}
	v.File = file
	for _, s := range presence.Styles {
		v.Styles = append(v.Styles, StyleInfo{Name: s.Name, Previews: previews(s.Style)})
	}
	return v
}

func (a *App) Save(c config.Config) error {
	return config.Save(a.configPath(), c)
}

type DaemonStatus struct {
	Running bool             `json:"running"`
	Error   string           `json:"error,omitempty"`
	Status  *status.Response `json:"status,omitempty"`
}

func (a *App) Status() DaemonStatus {
	r, err := status.Fetch(a.ctx, a.statusPort)
	if err != nil {
		return DaemonStatus{Error: err.Error()}
	}
	return DaemonStatus{Running: true, Status: &r}
}

func (a *App) StartDaemon() error            { return startDaemon() }
func (a *App) RestartDaemon() error          { return restartDaemon() }
func (a *App) ServiceInfo() ServiceInfo      { return serviceInfo() }
func (a *App) SetStartAtLogin(on bool) error { return setStartAtLogin(on) }

// Versions for the Advanced section; the mod's is from its last launch.
func (a *App) AppVersion() string { return version }

type Location struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Short  string `json:"short"` // Path with the home folder as ~
	Exists bool   `json:"exists"`
}

// Locations are the folders worth knowing about. Path is empty when unknown,
// e.g. the game folder for an install that didn't use install.sh.
func (a *App) Locations() []Location {
	game := installedGameDir()
	under := func(parts ...string) string {
		if game == "" {
			return ""
		}
		return filepath.Join(append([]string{game}, parts...)...)
	}
	programs := ""
	if self, err := os.Executable(); err == nil {
		programs = filepath.Dir(self)
	}
	locs := []Location{
		{Key: "plugin", Label: "Mod", Path: under("BepInEx", "plugins", "UnbeatableRPC")},
		{Key: "bepinex", Label: "BepInEx (logs, config)", Path: under("BepInEx")},
		{Key: "game", Label: "Game", Path: game},
		{Key: "settings", Label: "Settings", Path: filepath.Dir(a.configPath())},
		{Key: "programs", Label: "Programs", Path: programs},
	}
	home, _ := os.UserHomeDir()
	for i := range locs {
		if locs[i].Path != "" {
			_, err := os.Stat(locs[i].Path)
			locs[i].Exists = err == nil
			locs[i].Short = locs[i].Path
			if rel, err := filepath.Rel(home, locs[i].Path); home != "" && err == nil && !strings.HasPrefix(rel, "..") {
				locs[i].Short = filepath.Join("~", rel)
			}
		}
	}
	return locs
}

// Browse opens one of Locations in the file manager.
func (a *App) Browse(key string) error {
	for _, l := range a.Locations() {
		if l.Key == key {
			if !l.Exists {
				return fmt.Errorf("%s doesn't exist", l.Path)
			}
			return openFolder(l.Path)
		}
	}
	return fmt.Errorf("unknown location %q", key)
}

// Uninstall asks for confirmation, runs the installed uninstaller and quits.
// It returns false if the user cancelled.
func (a *App) Uninstall(all bool) (bool, error) {
	path, err := uninstallerPath()
	if err != nil {
		return false, err
	}
	msg := "This removes the plugin, the daemon and its service, this app, and your settings."
	if all {
		msg += "\n\nIt also removes BepInEx and undoes the Proton prefix change, if the installer made them."
	}
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "Uninstall UNBEATABLE RPC?",
		Message: msg,
	})
	if err != nil {
		return false, err
	}
	if choice != "Yes" {
		return false, nil
	}

	out, err := runUninstaller(path, all)
	if err != nil {
		return false, fmt.Errorf("uninstall failed: %s", lastLines(out, 3))
	}
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:  runtime.InfoDialog,
		Title: "Uninstalled",
		Message: "UNBEATABLE RPC has been removed.\n\n" +
			"If you added anything to UNBEATABLE's Steam launch options for this mod, remove it there too.",
	})
	runtime.Quit(a.ctx)
	return true, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (a *App) configPath() string {
	if r, err := status.Fetch(a.ctx, a.statusPort); err == nil && r.ConfigPath != "" {
		return r.ConfigPath
	}
	p, _ := config.Path()
	return p
}

var samples = []struct {
	where string
	state presence.State
}{
	{"Song", presence.State{V: 1, State: presence.StateArcade, Song: "RE-TUNED", Artist: "peak divide ft. Rachel Lake",
		Charter: "re-kouned", Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), Rank: "B"}},
	{"Story", presence.State{V: 1, State: presence.StateStory, Chapter: "Chapter 4", Stars: ptr(8.703)}},
	{"Menus", presence.State{V: 1, State: presence.StateIdle, Stars: ptr(8.703)}},
}

func previews(style presence.Style) []Preview {
	var out []Preview
	for _, s := range samples {
		act := presence.Format(s.state, presence.Options{Style: style})
		out = append(out, Preview{Where: s.where, Details: act.Details, State: act.State})
	}
	return out
}

func ptr[T any](v T) *T { return &v }
