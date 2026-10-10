package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
)

type Manager struct {
	Options          Options
	MinInterval      time.Duration
	HeartbeatTimeout time.Duration
	Log              *log.Logger
	Verbose          bool

	Dial func(ctx context.Context) (*discordipc.Client, error)

	desired   *discordipc.Activity
	sent      *discordipc.Activity
	sentValid bool
	lastSend  time.Time
	lastHeard time.Time
	gameAlive bool

	client    *discordipc.Client
	nextDial  time.Time
	backoff   time.Duration
	dialFails int
	lastState State
	lastError string

	mu      sync.Mutex
	pending *Update
	wake    chan struct{}
	status  Status
}

// Update changes the manager's settings while it runs.
type Update struct {
	Options Options
	Verbose bool
	// Reconnect drops the Discord connection so the next one uses new dial
	// settings (client ID, socket).
	Reconnect bool
}

// Status is what the manager is doing, for the status endpoint.
type Status struct {
	GameRunning bool                 `json:"gameRunning"`
	LastHeard   *time.Time           `json:"lastHeard,omitempty"`
	Discord     DiscordStatus        `json:"discord"`
	Activity    *discordipc.Activity `json:"activity"`
}

type DiscordStatus struct {
	Connected bool   `json:"connected"`
	Socket    string `json:"socket,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
)

var tick = 500 * time.Millisecond

func (m *Manager) Run(ctx context.Context, states <-chan State) {
	if m.MinInterval == 0 {
		m.MinInterval = 4 * time.Second
	}
	if m.HeartbeatTimeout == 0 {
		m.HeartbeatTimeout = 30 * time.Second
	}
	if m.Log == nil {
		m.Log = log.Default()
	}
	m.backoff = minBackoff

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	defer m.publish()
	defer m.disconnect(true)

	for {
		var done <-chan struct{}
		if m.client != nil {
			done = m.client.Done()
		}
		select {
		case <-ctx.Done():
			return
		case s := <-states:
			m.handle(s, time.Now())
		case <-done:
			m.Log.Printf("discord connection lost: %v", m.client.Err())
			m.client = nil
			m.nextDial = time.Now().Add(m.backoff)
		case <-m.wakeCh():
			m.mu.Lock()
			u := m.pending
			m.pending = nil
			m.mu.Unlock()
			if u != nil {
				m.apply(*u)
			}
		case <-ticker.C:
		}
		m.reconcile(ctx, time.Now())
		m.publish()
	}
}

// Apply hands new settings to a running manager.
func (m *Manager) Apply(u Update) {
	m.mu.Lock()
	m.pending = &u
	m.mu.Unlock()
	select {
	case m.wakeCh() <- struct{}{}:
	default:
	}
}

// Status returns a snapshot of what the manager is doing.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) wakeCh() chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.wake == nil {
		m.wake = make(chan struct{}, 1)
	}
	return m.wake
}

func (m *Manager) apply(u Update) {
	m.Options = u.Options
	m.Verbose = u.Verbose
	if u.Reconnect {
		m.disconnect(true)
		m.nextDial = time.Time{}
		m.backoff = minBackoff
		m.dialFails = 0
	}
	if m.gameAlive {
		m.desired = Format(m.lastState, m.Options)
	}
}

func (m *Manager) publish() {
	st := Status{GameRunning: m.gameAlive, Discord: DiscordStatus{LastError: m.lastError}}
	if !m.lastHeard.IsZero() {
		t := m.lastHeard
		st.LastHeard = &t
	}
	if m.client != nil {
		st.Discord.Connected = true
		st.Discord.Socket = m.client.Path
	}
	if m.sentValid && m.sent != nil {
		a := *m.sent
		st.Activity = &a
	}
	m.mu.Lock()
	m.status = st
	m.mu.Unlock()
}

func (m *Manager) handle(s State, now time.Time) {
	m.lastHeard = now
	if s.State == StateClosed {
		if m.gameAlive {
			m.Log.Printf("game closed")
		}
		m.gameAlive = false
		m.desired = nil
		return
	}
	if !m.gameAlive {
		m.Log.Printf("game is running")
	}
	m.gameAlive = true
	m.lastState = s
	m.desired = Format(s, m.Options)
}

func (m *Manager) reconcile(ctx context.Context, now time.Time) {
	if m.gameAlive && now.Sub(m.lastHeard) > m.HeartbeatTimeout {
		m.Log.Printf("no heartbeat for %s, assuming the game has closed", m.HeartbeatTimeout)
		m.gameAlive = false
		m.desired = nil
	}

	if !m.gameAlive {
		m.disconnect(true)
		return
	}

	if m.client == nil {
		if now.Before(m.nextDial) {
			return
		}
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		c, err := m.Dial(dctx)
		cancel()
		if err != nil {
			m.dialFails++
			m.lastError = err.Error()
			if m.dialFails == 1 || m.Verbose {
				m.Log.Printf("cannot reach Discord (retrying with backoff): %v", err)
			}
			m.nextDial = now.Add(m.backoff)
			m.backoff = min(m.backoff*2, maxBackoff)
			return
		}
		m.Log.Printf("connected to %s", c.Path)
		m.client = c
		m.dialFails = 0
		m.lastError = ""
		m.backoff = minBackoff
		m.sentValid = false
	}

	if m.sentValid && sameActivity(m.sent, m.desired) {
		return
	}
	if now.Sub(m.lastSend) < m.MinInterval {
		return
	}
	if err := m.client.SetActivity(ctx, m.desired); err != nil {
		m.Log.Printf("SET_ACTIVITY failed: %v", err)
		m.lastError = "SET_ACTIVITY: " + err.Error()
		m.client.Close()
		m.client = nil
		m.nextDial = now.Add(m.backoff)
		return
	}
	m.sent, m.sentValid, m.lastSend = m.desired, true, now
	if m.Verbose {
		b, _ := json.Marshal(m.desired)
		m.Log.Printf("activity set: %s", b)
	}
}

func (m *Manager) disconnect(clear bool) {
	if m.client == nil {
		return
	}
	if clear {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = m.client.ClearActivity(ctx)
		cancel()
	}
	m.client.Close()
	m.client = nil
	m.sentValid = false
	m.Log.Printf("activity cleared, disconnected from Discord")
}

func sameActivity(a, b *discordipc.Activity) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return bytes.Equal(ja, jb)
}
