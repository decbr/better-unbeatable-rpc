package main

import (
	"context"
	"fmt"
	"os"
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
	for _, name := range presence.StyleNames() {
		v.Styles = append(v.Styles, StyleInfo{Name: name, Previews: previews(presence.Styles[name])})
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

func (a *App) StartDaemon() error { return startDaemon() }

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
		Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), Rank: "B"}},
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
