package mcu

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

// Manager tracks active MCU sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

var defaultManager = &Manager{sessions: make(map[string]*Session)}

// DefaultManager returns the process-wide session manager.
func DefaultManager() *Manager {
	return defaultManager
}

// GetByPort returns an active session bound to port, if any.
func (m *Manager) GetByPort(port string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.Port == port {
			return s, true
		}
	}
	return nil, false
}

// Connect opens a port, configures the MCU, and starts a session.
// If the port already has an active session, that session is returned.
func (m *Manager) Connect(port string, baud int, timeout time.Duration, build cfgtranslate.BuildResult) (*Session, error) {
	if existing, ok := m.GetByPort(port); ok {
		return existing, nil
	}

	if baud == 0 {
		baud = 250000
	}
	if timeout == 0 {
		timeout = DefaultProbeTimeout
	}

	nextOID := cfgtranslate.NextOID(build.Commands)
	stopTrsyncOIDs := make(map[int]int)
	var trsyncCommands []string
	for _, obj := range build.Objects {
		if obj.Kind != cfgtranslate.KindStepper {
			continue
		}
		stopTrsyncOIDs[obj.OID] = nextOID
		trsyncCommands = append(trsyncCommands, fmt.Sprintf("config_trsync oid=%d", nextOID))
		nextOID++
	}
	commands := append(build.Commands, trsyncCommands...)

	conn, err := OpenConn(port, baud, timeout)
	if err != nil {
		return nil, err
	}

	lines := cfgtranslate.FinalizeCommands(commands)
	if err := conn.ConfigureWithInit(lines, build.InitCommands); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := conn.SyncClock(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("sync clock: %w", err)
	}

	id, err := newSessionID()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	session := NewSession(id, port, baud, conn, build.Objects)
	session.stopTrsyncOIDs = stopTrsyncOIDs
	session.ClockBase = conn.clockBase
	session.ClockBaseAt = conn.clockBaseAt

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()
	return session, nil
}

// Get returns a session by ID.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Disconnect closes and removes a session.
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found")
	}
	return session.Close()
}

// List returns all active sessions.
func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func newSessionID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
