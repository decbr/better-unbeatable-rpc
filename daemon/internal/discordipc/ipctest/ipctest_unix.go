//go:build !windows

// Package ipctest makes discord-ipc listeners for tests.
package ipctest

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// Listen listens on a new discord-ipc path, closed when the test ends.
func Listen(t *testing.T) (string, net.Listener) {
	t.Helper()
	// Keep the path short: unix socket paths are limited to ~108 bytes.
	dir, err := os.MkdirTemp("", "dipc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "discord-ipc-0")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return path, ln
}

// Stale is a socket path with nothing listening, like a crashed Discord leaves.
func Stale(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dipc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "discord-ipc-1")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	return path
}
