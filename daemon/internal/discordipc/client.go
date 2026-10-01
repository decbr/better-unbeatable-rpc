package discordipc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type Activity struct {
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	Timestamps *Timestamps `json:"timestamps,omitempty"`
	Assets     *Assets     `json:"assets,omitempty"`
}

type Timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

var ErrClosed = errors.New("discordipc: connection closed")

const requestTimeout = 5 * time.Second

type Client struct {
	conn net.Conn
	Path string

	writeMu sync.Mutex

	mu      sync.Mutex
	pending map[string]chan message
	err     error

	done chan struct{}
}

type message struct {
	Cmd   string          `json:"cmd"`
	Evt   string          `json:"evt"`
	Nonce string          `json:"nonce"`
	Data  json.RawMessage `json:"data"`
}

func Dial(ctx context.Context, clientID string, paths ...string) (*Client, error) {
	if len(paths) == 0 {
		paths = CandidatePaths()
	}
	if len(paths) == 0 {
		return nil, errors.New("discordipc: no discord-ipc socket found (is Vesktop running with arRPC on?)")
	}
	var errs []error
	for _, p := range paths {
		c, err := dialOne(ctx, clientID, p)
		if err == nil {
			return c, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", p, err))
	}
	return nil, errors.Join(errs...)
}

func dialOne(ctx context.Context, clientID, path string) (*Client, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(requestTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	hs, _ := json.Marshal(map[string]any{"v": 1, "client_id": clientID})
	if err := WriteFrame(conn, OpHandshake, hs); err != nil {
		conn.Close()
		return nil, err
	}
	for {
		op, payload, err := ReadFrame(conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("waiting for READY: %w", err)
		}
		switch op {
		case OpPing:
			if err := WriteFrame(conn, OpPong, payload); err != nil {
				conn.Close()
				return nil, err
			}
			continue
		case OpClose:
			conn.Close()
			return nil, fmt.Errorf("handshake rejected: %s", payload)
		case OpFrame:
			var m message
			if json.Unmarshal(payload, &m) == nil && m.Cmd == "DISPATCH" && m.Evt == "READY" {
				_ = conn.SetDeadline(time.Time{})
				c := &Client{conn: conn, Path: path, pending: map[string]chan message{}, done: make(chan struct{})}
				go c.readLoop()
				return c, nil
			}
		}
	}
}

func (c *Client) readLoop() {
	var err error
	defer func() { c.fail(err) }()
	for {
		var op Opcode
		var payload []byte
		op, payload, err = ReadFrame(c.conn)
		if err != nil {
			return
		}
		switch op {
		case OpPing:
			if err = c.write(OpPong, payload); err != nil {
				return
			}
		case OpClose:
			err = fmt.Errorf("%w: server sent CLOSE: %s", ErrClosed, payload)
			return
		case OpFrame:
			var m message
			if json.Unmarshal(payload, &m) != nil || m.Nonce == "" {
				continue
			}
			c.mu.Lock()
			ch := c.pending[m.Nonce]
			delete(c.pending, m.Nonce)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	if err == nil {
		err = ErrClosed
	}
	c.err = err
	c.conn.Close()
	close(c.done)
}

func (c *Client) write(op Opcode, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteFrame(c.conn, op, payload)
}

func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Client) Close() error {
	_ = c.write(OpClose, []byte(`{}`))
	c.fail(ErrClosed)
	return nil
}

func (c *Client) SetActivity(ctx context.Context, a *Activity) error {
	args := map[string]any{"pid": os.Getpid(), "activity": a}
	_, err := c.request(ctx, "SET_ACTIVITY", args)
	return err
}

func (c *Client) ClearActivity(ctx context.Context) error { return c.SetActivity(ctx, nil) }

func (c *Client) request(ctx context.Context, cmd string, args any) (message, error) {
	nonce := newNonce()
	payload, err := json.Marshal(map[string]any{"cmd": cmd, "args": args, "nonce": nonce})
	if err != nil {
		return message{}, err
	}
	ch := make(chan message, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return message{}, c.err
	}
	c.pending[nonce] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, nonce)
		c.mu.Unlock()
	}()

	if err := c.write(OpFrame, payload); err != nil {
		c.fail(err)
		return message{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	select {
	case m := <-ch:
		if m.Evt == "ERROR" {
			return m, fmt.Errorf("discordipc: %s failed: %s", cmd, m.Data)
		}
		return m, nil
	case <-c.done:
		return message{}, c.Err()
	case <-ctx.Done():
		return message{}, fmt.Errorf("discordipc: %s: %w", cmd, ctx.Err())
	}
}

func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
