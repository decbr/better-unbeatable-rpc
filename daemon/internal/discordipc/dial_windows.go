package discordipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

const noSocketHint = "no discord-ipc pipe found (is Discord running?)"

func dialConn(ctx context.Context, path string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, path)
}
