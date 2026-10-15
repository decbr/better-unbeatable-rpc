package ipctest

import (
	"crypto/rand"
	"fmt"
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
)

// Listen listens on a new discord-ipc path, closed when the test ends.
func Listen(t *testing.T) (string, net.Listener) {
	t.Helper()
	path := pipeName()
	ln, err := winio.ListenPipe(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return path, ln
}

// Stale is a pipe name nothing is listening on.
func Stale(t *testing.T) string { return pipeName() }

func pipeName() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf(`\\.\pipe\dipc-test-%x`, b)
}
