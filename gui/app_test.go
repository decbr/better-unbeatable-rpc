package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
)

// newTestApp has no daemon to talk to, and its settings file in a temp dir.
func newTestApp(t *testing.T) *App {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return &App{ctx: context.Background(), statusPort: port}
}

func TestLoadPreviews(t *testing.T) {
	v := newTestApp(t).Load()
	if v.FileError != "" || !reflect.DeepEqual(v.File, config.Config{}) {
		t.Errorf("fresh load: file %+v, error %q", v.File, v.FileError)
	}
	got := map[string]Preview{}
	for _, s := range v.Styles {
		got[s.Name] = s.Previews[0]
	}
	if p := got["spotify"]; p.Details != "RE-TUNED" || p.State != "peak divide ft. Rachel Lake" {
		t.Errorf("spotify song preview = %+v", p)
	}
	if p := got["detailed"]; p.Details != "RE-TUNED - peak divide ft. Rachel Lake" {
		t.Errorf("detailed song preview = %+v", p)
	}
}

func TestSaveWritesSettingsFile(t *testing.T) {
	a := newTestApp(t)
	want := config.Config{Style: "spotify", Verbose: true}
	if err := a.Save(want); err != nil {
		t.Fatal(err)
	}
	path, _ := config.Path()
	if filepath.Dir(filepath.Dir(path)) != os.Getenv("XDG_CONFIG_HOME") {
		t.Fatalf("settings path %s is outside the test dir", path)
	}
	if got, err := config.Read(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("read back %+v, %v; want %+v", got, err, want)
	}
	if v := a.Load(); !reflect.DeepEqual(v.File, want) {
		t.Errorf("Load after Save = %+v", v.File)
	}
}

func TestStatusWithoutDaemon(t *testing.T) {
	if st := newTestApp(t).Status(); st.Running || st.Error == "" {
		t.Errorf("got %+v, want not running with an error", st)
	}
}

func TestRunUninstaller(t *testing.T) {
	script := filepath.Join(t.TempDir(), "unbeatable-rpc-uninstall")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"args: $*\"\n[ \"$1\" != fail ] || { echo 'install.sh: UNBEATABLE is running; quit it first'; exit 1; }\n"), 0o755)

	if out, err := runUninstaller(script, false); err != nil || out != "args: \n" {
		t.Errorf("plain: %q, %v", out, err)
	}
	if out, err := runUninstaller(script, true); err != nil || out != "args: --all\n" {
		t.Errorf("--all: %q, %v", out, err)
	}
	out, err := runUninstaller(script+"-missing", false)
	if err == nil {
		t.Errorf("missing uninstaller ran: %q", out)
	}
	if got := lastLines("a\nb\nc\nd\n", 2); got != "c\nd" {
		t.Errorf("lastLines = %q", got)
	}
}

func TestInstalledGameDirReadsInstallerState(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if got := installedGameDir(); got != "" {
		t.Errorf("no state file: got %q, want empty", got)
	}
	// the installer writes it with printf %q, which escapes spaces and quotes
	game := filepath.Join(t.TempDir(), "Steam Library", "it's UNBEATABLE")
	os.MkdirAll(filepath.Join(state, "unbeatable-rpc"), 0o755)
	script := `printf 'GAME_DIR=%q\nSERVICE=%q\n' "$1" true >"$2"`
	if err := exec.Command("bash", "-c", script, "_", game, filepath.Join(state, "unbeatable-rpc", "install.env")).Run(); err != nil {
		t.Fatal(err)
	}
	if got := installedGameDir(); got != game {
		t.Errorf("got %q, want %q", got, game)
	}
}

func TestLocations(t *testing.T) {
	a := newTestApp(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	byKey := map[string]Location{}
	for _, l := range a.Locations() {
		byKey[l.Key] = l
	}
	if l := byKey["game"]; l.Path != "" || l.Exists {
		t.Errorf("game without installer state = %+v, want unknown", l)
	}
	if l := byKey["plugin"]; l.Path != "" {
		t.Errorf("plugin without a game folder = %+v", l)
	}
	if l := byKey["settings"]; l.Path == "" {
		t.Error("settings folder missing")
	}
	if err := a.Browse("nope"); err == nil {
		t.Error("Browse accepted an unknown key")
	}
	if err := a.Browse("game"); err == nil {
		t.Error("Browse opened a folder that doesn't exist")
	}
}

func TestLoadKeepsRegistryOrder(t *testing.T) {
	var got []string
	for _, s := range newTestApp(t).Load().Styles {
		got = append(got, s.Name)
	}
	if want := presence.StyleNames(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("app lists %v, registry order is %v", got, want)
	}
}

func TestCustomStyles(t *testing.T) {
	a := newTestApp(t)
	mine := presence.CustomStyle{Name: "mine", Song: presence.Template{Details: "{title} by {artist}"}}
	broken := presence.CustomStyle{Name: "broken", Song: presence.Template{Details: "{titel}"}}

	if err := a.CheckCustomStyles([]presence.CustomStyle{mine, {Name: "osu"}}); err == nil {
		t.Error("accepted a custom style named after a built-in")
	}
	if err := a.CheckCustomStyles([]presence.CustomStyle{mine}); err != nil {
		t.Error(err)
	}
	// a hand-edited file can hold a broken one: it's listed with its error
	if err := a.Save(config.Config{Style: "mine", CustomStyles: []presence.CustomStyle{mine, broken}}); err != nil {
		t.Fatal(err)
	}

	v := a.Load()
	byName := map[string]StyleInfo{}
	for _, s := range v.Styles {
		byName[s.Name] = s
	}
	if s := byName["mine"]; !s.Custom || s.Previews[0].Details != "RE-TUNED by peak divide ft. Rachel Lake" {
		t.Errorf("mine = %+v", s)
	}
	if s := byName["broken"]; s.Error == "" || s.Previews != nil {
		t.Errorf("broken = %+v", s)
	}
	if s := byName["osu"]; s.Custom || s.Template.Song.Details == "" {
		t.Errorf("built-in osu = %+v, want its template", s)
	}

	ps, err := a.PreviewCustom(mine)
	if err != nil || len(ps) != len(samples) || ps[1].Where != "Results" || ps[1].Details != "YEAH YEAH by peak divide" {
		t.Errorf("PreviewCustom = %+v, %v", ps, err)
	}
	if _, err := a.PreviewCustom(broken); err == nil || !strings.Contains(err.Error(), "{titel}") {
		t.Errorf("PreviewCustom(broken) error = %v", err)
	}
}

func timeNowPlus(t *testing.T, seconds int) time.Time {
	t.Helper()
	return time.Now().Add(time.Duration(seconds) * time.Second)
}
