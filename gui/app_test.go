package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
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
	if v.FileError != "" || v.File != (config.Config{}) {
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
	if got, err := config.Read(path); err != nil || got != want {
		t.Errorf("read back %+v, %v; want %+v", got, err, want)
	}
	if v := a.Load(); v.File != want {
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
