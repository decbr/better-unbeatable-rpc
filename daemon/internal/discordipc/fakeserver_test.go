package discordipc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeServer struct {
	t    *testing.T
	path string
	ln   net.Listener

	mu         sync.Mutex
	handshake  map[string]any
	activities []json.RawMessage
	gotPong    bool
	rejectNext bool
	conns      []net.Conn
}

func newFakeServer(t *testing.T) *fakeServer {
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
	s := &fakeServer{t: t, path: path, ln: ln}
	t.Cleanup(func() { ln.Close() })
	go s.serve()
	return s
}

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeServer) dropAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.Close()
	}
	s.conns = nil
}

func (s *fakeServer) handle(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	s.conns = append(s.conns, conn)
	s.mu.Unlock()
	op, payload, err := ReadFrame(conn)
	if err != nil || op != OpHandshake {
		return
	}
	var hs map[string]any
	_ = json.Unmarshal(payload, &hs)
	s.mu.Lock()
	s.handshake = hs
	s.mu.Unlock()

	_ = WriteFrame(conn, OpFrame, []byte(`{"cmd":"DISPATCH","evt":"READY","data":{"v":1,"user":{"id":"1","username":"test"}}}`))
	_ = WriteFrame(conn, OpPing, []byte(`{"n":1}`))

	for {
		op, payload, err := ReadFrame(conn)
		if err != nil {
			return
		}
		switch op {
		case OpPong:
			s.mu.Lock()
			s.gotPong = true
			s.mu.Unlock()
		case OpClose:
			return
		case OpFrame:
			var req struct {
				Cmd   string `json:"cmd"`
				Nonce string `json:"nonce"`
				Args  struct {
					Pid      int             `json:"pid"`
					Activity json.RawMessage `json:"activity"`
				} `json:"args"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				s.t.Errorf("bad request JSON: %v", err)
				return
			}
			if req.Cmd != "SET_ACTIVITY" || req.Nonce == "" || req.Args.Pid != os.Getpid() {
				s.t.Errorf("unexpected request: %s", payload)
			}
			s.mu.Lock()
			reject := s.rejectNext
			s.rejectNext = false
			if !reject {
				s.activities = append(s.activities, req.Args.Activity)
			}
			s.mu.Unlock()
			var resp []byte
			if reject {
				resp, _ = json.Marshal(map[string]any{"cmd": "SET_ACTIVITY", "evt": "ERROR", "nonce": req.Nonce, "data": map[string]any{"code": 4000, "message": "nope"}})
			} else {
				resp, _ = json.Marshal(map[string]any{"cmd": "SET_ACTIVITY", "nonce": req.Nonce, "data": json.RawMessage(req.Args.Activity)})
			}
			_ = WriteFrame(conn, OpFrame, resp)
		}
	}
}

func TestClientHandshakeAndSetActivity(t *testing.T) {
	srv := newFakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Dial(ctx, "1234", srv.path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	a := &Activity{Details: "RETUNED AC - Normal", State: "PB: 1,166,540 (S)", Timestamps: &Timestamps{Start: 1727790000}}
	if err := c.SetActivity(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearActivity(ctx); err != nil {
		t.Fatal(err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.handshake["client_id"] != "1234" || srv.handshake["v"] != float64(1) {
		t.Errorf("handshake = %v", srv.handshake)
	}
	if !srv.gotPong {
		t.Error("client did not answer PING with PONG")
	}
	if len(srv.activities) != 2 {
		t.Fatalf("got %d SET_ACTIVITY calls, want 2", len(srv.activities))
	}
	var got Activity
	if err := json.Unmarshal(srv.activities[0], &got); err != nil {
		t.Fatal(err)
	}
	if got.Details != a.Details || got.State != a.State || got.Timestamps == nil || got.Timestamps.Start != 1727790000 {
		t.Errorf("activity = %+v", got)
	}
	if string(srv.activities[1]) != "null" {
		t.Errorf("clear sent %s, want null", srv.activities[1])
	}
}

func TestClientReportsErrorEvent(t *testing.T) {
	srv := newFakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, "1234", srv.path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	srv.mu.Lock()
	srv.rejectNext = true
	srv.mu.Unlock()
	if err := c.SetActivity(ctx, &Activity{Details: "x"}); err == nil {
		t.Error("expected an error for an ERROR event")
	}
}

func TestDialSkipsStaleSocket(t *testing.T) {
	srv := newFakeServer(t)

	dir, _ := os.MkdirTemp("", "dipc")
	defer os.RemoveAll(dir)
	stale := filepath.Join(dir, "discord-ipc-1")
	ln, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, "1234", stale, srv.path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Path != srv.path {
		t.Errorf("connected to %s, want %s", c.Path, srv.path)
	}
}

func TestDoneClosesWhenServerGoes(t *testing.T) {
	srv := newFakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, "1234", srv.path)
	if err != nil {
		t.Fatal(err)
	}
	srv.dropAll()
	select {
	case <-c.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("Done not closed after the connection ended")
	}
	if c.Err() == nil {
		t.Error("Err() is nil after the connection died")
	}
}
