package main

import (
	"strings"
	"testing"
)

func TestDiagnosticsSections(t *testing.T) {
	a := newTestApp(t)
	isolateState(t)
	d := a.diagnostics()
	for _, want := range []string{"UNBEATABLE RPC diagnostics", "App dev, daemon not running", "== Checks",
		"== Daemon status", "unreachable", "== Settings file", "== Mod settings", "== Daemon log", "== BepInEx log"} {
		if !strings.Contains(d, want) {
			t.Errorf("missing %q in:\n%s", want, d)
		}
	}
}
