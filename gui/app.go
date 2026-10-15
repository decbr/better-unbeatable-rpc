package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
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
	platform   platformState
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
	// Placeholders are the {names} custom style templates can use.
	Placeholders []string `json:"placeholders"`
}

// StyleInfo is a built-in style, then the custom ones from the file.
type StyleInfo struct {
	Name     string               `json:"name"`
	Custom   bool                 `json:"custom"`
	Template presence.CustomStyle `json:"template"` // a built-in's is the starting point for a copy
	Previews []Preview            `json:"previews"`
	Error    string               `json:"error,omitempty"` // a custom style that doesn't compile
}

type Preview struct {
	Where   string `json:"where"`
	Details string `json:"details"`
	State   string `json:"state"`
}

func (a *App) Load() View {
	v := View{Defaults: config.Defaults(), EnvVars: config.EnvVars, Placeholders: presence.Placeholders}
	v.ConfigPath = a.configPath()
	file, err := config.Read(v.ConfigPath)
	if err != nil {
		v.FileError = err.Error()
	}
	v.File = file
	for _, s := range presence.Styles {
		v.Styles = append(v.Styles, StyleInfo{Name: s.Name, Template: s.Template, Previews: previews(s.Style, cardSamples)})
	}
	for _, c := range file.CustomStyles {
		info := StyleInfo{Name: c.Name, Custom: true, Template: c}
		if style, err := c.Compile(); err != nil {
			info.Error = err.Error()
		} else {
			info.Previews = previews(style, cardSamples)
		}
		v.Styles = append(v.Styles, info)
	}
	return v
}

// Save doesn't check custom styles, so a broken one in a hand-edited file
// doesn't stop other settings being saved: the editor calls CheckCustomStyles.
func (a *App) Save(c config.Config) error {
	return config.Save(a.configPath(), c)
}

func (a *App) CheckCustomStyles(styles []presence.CustomStyle) error {
	return presence.ValidateCustom(styles)
}

// PreviewCustom is the style editor's live preview, in every state.
func (a *App) PreviewCustom(c presence.CustomStyle) ([]Preview, error) {
	style, err := c.Compile()
	if err != nil {
		return nil, err
	}
	return previews(style, samples), nil
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
func (a *App) DaemonLog() string             { return daemonLog(200) }

// Versions for the Advanced section; the mod's is from its last launch.
func (a *App) AppVersion() string { return version }

// Platform is "linux" or "windows", for the page's OS-specific wording.
func (a *App) Platform() string { return goruntime.GOOS }

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
		msg += "\n\n" + uninstallAllText
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
	if !uninstallerAfterExit {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "Uninstalled",
			Message: "UNBEATABLE RPC has been removed.\n\n" + uninstalledText,
		})
	}
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

type sample struct {
	where string
	state presence.State
}

var (
	songSample = sample{"Song", presence.State{V: 1, State: presence.StateArcade, Song: "RE-TUNED", Artist: "peak divide ft. Rachel Lake",
		Charter: "re-kouned", Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), Rank: "B", Stars: ptr(8.703)}}
	storySample = sample{"Story", presence.State{V: 1, State: presence.StateStory, Chapter: "Chapter 4", Stars: ptr(8.703)}}
	menusSample = sample{"Menus", presence.State{V: 1, State: presence.StateIdle, Stars: ptr(8.703)}}

	// cardSamples are shown on each style's card; samples, every state, in the editor.
	cardSamples = []sample{songSample, storySample, menusSample}
	samples     = []sample{
		songSample,
		{"Results", presence.State{V: 1, State: presence.StateResults, Song: "YEAH YEAH", Artist: "peak divide",
			Charter: "re-kouned", Difficulty: "Star", Level: ptr(14), HighScore: ptr(1256914), Rank: "A", Stars: ptr(8.703)}},
		{"Story song", presence.State{V: 1, State: presence.StateStory, Chapter: "Chapter 2", Song: "Low", Artist: "peak divide",
			Difficulty: "Normal", Stars: ptr(8.703)}},
		storySample,
		menusSample,
	}
)

func previews(style presence.Style, samples []sample) []Preview {
	var out []Preview
	for _, s := range samples {
		act := presence.Format(s.state, presence.Options{Style: style})
		out = append(out, Preview{Where: s.where, Details: act.Details, State: act.State})
	}
	return out
}

func ptr[T any](v T) *T { return &v }
