package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
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
		case <-ticker.C:
		}
		m.reconcile(ctx, time.Now())
	}
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
