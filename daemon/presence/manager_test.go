package presence

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
)

type ipcRecorder struct {
	path string
	ln   net.Listener

	mu      sync.Mutex
	got     []string
	conns   []net.Conn
	accepts int
}

func newRecorder(t *testing.T) *ipcRecorder {
	dir, err := os.MkdirTemp("", "pres")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	r := &ipcRecorder{path: filepath.Join(dir, "discord-ipc-0")}
	r.ln, err = net.Listen("unix", r.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.ln.Close() })
	go func() {
		for {
			c, err := r.ln.Accept()
			if err != nil {
				return
			}
			r.mu.Lock()
			r.conns = append(r.conns, c)
			r.accepts++
			r.mu.Unlock()
			go r.serve(c)
		}
	}()
	return r
}

func (r *ipcRecorder) serve(c net.Conn) {
	defer c.Close()
	if _, _, err := discordipc.ReadFrame(c); err != nil {
		return
	}
	_ = discordipc.WriteFrame(c, discordipc.OpFrame, []byte(`{"cmd":"DISPATCH","evt":"READY","data":{}}`))
	for {
		op, payload, err := discordipc.ReadFrame(c)
		if err != nil || op == discordipc.OpClose {
			return
		}
		var req struct {
			Nonce string `json:"nonce"`
			Args  struct {
				Activity *discordipc.Activity `json:"activity"`
			} `json:"args"`
		}
		_ = json.Unmarshal(payload, &req)
		details := ""
		if req.Args.Activity != nil {
			details = req.Args.Activity.Details
		}
		r.mu.Lock()
		r.got = append(r.got, details)
		r.mu.Unlock()
		resp, _ := json.Marshal(map[string]any{"cmd": "SET_ACTIVITY", "nonce": req.Nonce})
		_ = discordipc.WriteFrame(c, discordipc.OpFrame, resp)
	}
}

func (r *ipcRecorder) snapshot() ([]string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.got...), r.accepts
}

func (r *ipcRecorder) dropAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.conns {
		c.Close()
	}
	r.conns = nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func startManager(t *testing.T, r *ipcRecorder, minInterval, heartbeat time.Duration) chan<- State {
	_, states := startManagerWith(t, r, minInterval, heartbeat)
	return states
}

func startManagerWith(t *testing.T, r *ipcRecorder, minInterval, heartbeat time.Duration) (*Manager, chan<- State) {
	old := tick
	tick = 20 * time.Millisecond
	t.Cleanup(func() { tick = old })

	m := &Manager{
		MinInterval:      minInterval,
		HeartbeatTimeout: heartbeat,
		Log:              log.New(io.Discard, "", 0),
		Dial: func(ctx context.Context) (*discordipc.Client, error) {
			return discordipc.Dial(ctx, "1", r.path)
		},
	}
	states := make(chan State, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx, states); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return m, states
}

func TestManagerCoalescesToLatest(t *testing.T) {
	r := newRecorder(t)
	states := startManager(t, r, 300*time.Millisecond, time.Minute)

	states <- State{V: 1, State: StateIdle}
	waitFor(t, "first activity", func() bool { got, _ := r.snapshot(); return len(got) == 1 })

	states <- State{V: 1, State: StateStory}
	states <- State{V: 1, State: StateArcade, Song: "Song A"}
	states <- State{V: 1, State: StateArcade, Song: "Song B"}
	waitFor(t, "coalesced activity", func() bool { got, _ := r.snapshot(); return len(got) == 2 })
	time.Sleep(400 * time.Millisecond)

	got, _ := r.snapshot()
	if len(got) != 2 || got[0] != "In menus" || got[1] != "Song B" {
		t.Errorf("sent %q, want [In menus, Song B]", got)
	}
}

func TestManagerClearsWhenHeartbeatStops(t *testing.T) {
	r := newRecorder(t)
	states := startManager(t, r, 10*time.Millisecond, 200*time.Millisecond)

	states <- State{V: 1, State: StateIdle}
	waitFor(t, "clear after heartbeat timeout", func() bool {
		got, _ := r.snapshot()
		return len(got) == 2 && got[0] == "In menus" && got[1] == ""
	})
}

func TestManagerClearsOnClosedMessage(t *testing.T) {
	r := newRecorder(t)
	states := startManager(t, r, 10*time.Millisecond, time.Minute)

	states <- State{V: 1, State: StateIdle}
	waitFor(t, "first activity", func() bool { got, _ := r.snapshot(); return len(got) == 1 })
	states <- State{V: 1, State: StateClosed}
	waitFor(t, "clear", func() bool { got, _ := r.snapshot(); return len(got) == 2 && got[1] == "" })
}

func TestManagerResendsAfterReconnect(t *testing.T) {
	r := newRecorder(t)
	states := startManager(t, r, 10*time.Millisecond, time.Minute)

	states <- State{V: 1, State: StateIdle}
	waitFor(t, "first activity", func() bool { got, _ := r.snapshot(); return len(got) == 1 })

	r.dropAll()
	go func() {
		for i := 0; i < 40; i++ {
			states <- State{V: 1, State: StateIdle}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	waitFor(t, "re-sent activity on a new connection", func() bool {
		got, accepts := r.snapshot()
		return accepts >= 2 && len(got) >= 2 && got[len(got)-1] == "In menus"
	})
}

func TestManagerApplyRestylesCurrentState(t *testing.T) {
	r := newRecorder(t)
	m, states := startManagerWith(t, r, 10*time.Millisecond, time.Minute)

	states <- State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide"}
	waitFor(t, "detailed activity", func() bool { got, _ := r.snapshot(); return len(got) == 1 })

	m.Apply(Update{Options: Options{Style: Spotify}})
	waitFor(t, "restyled activity without a new game message", func() bool { got, _ := r.snapshot(); return len(got) == 2 })

	got, accepts := r.snapshot()
	if got[0] != "RE-TUNED - peak divide" || got[1] != "RE-TUNED" {
		t.Errorf("sent %q, want [RE-TUNED - peak divide, RE-TUNED]", got)
	}
	if accepts != 1 {
		t.Errorf("connections = %d, want 1 (a style change shouldn't reconnect)", accepts)
	}
}

func TestManagerApplyReconnects(t *testing.T) {
	r := newRecorder(t)
	m, states := startManagerWith(t, r, 10*time.Millisecond, time.Minute)

	states <- State{V: 1, State: StateIdle}
	waitFor(t, "first activity", func() bool { got, _ := r.snapshot(); return len(got) == 1 })

	m.Apply(Update{Reconnect: true})
	waitFor(t, "second connection", func() bool { _, accepts := r.snapshot(); return accepts == 2 })
	waitFor(t, "activity resent on the new connection", func() bool {
		got, _ := r.snapshot()
		return len(got) >= 2 && got[len(got)-1] == "In menus"
	})
}

func TestManagerStatus(t *testing.T) {
	r := newRecorder(t)
	m, states := startManagerWith(t, r, 10*time.Millisecond, time.Minute)

	if st := m.Status(); st.GameRunning || st.Discord.Connected || st.Activity != nil {
		t.Errorf("before any message: %+v", st)
	}

	states <- State{V: 1, State: StateArcade, Song: "Low"}
	waitFor(t, "status with activity", func() bool {
		st := m.Status()
		return st.GameRunning && st.Discord.Connected && st.Activity != nil && st.Activity.Details == "Low"
	})
	if st := m.Status(); st.Discord.Socket != r.path || st.LastHeard == nil {
		t.Errorf("status = %+v, want socket %s and lastHeard set", st, r.path)
	}

	states <- State{V: 1, State: StateClosed}
	waitFor(t, "status after close", func() bool {
		st := m.Status()
		return !st.GameRunning && !st.Discord.Connected && st.Activity == nil
	})
}
