//go:build !windows

package discordipc

import (
	"context"
	"net"
)

const noSocketHint = "no discord-ipc socket found (is Vesktop running with arRPC on?)"

func dialConn(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}
