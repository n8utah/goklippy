package mcu

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

// Manager tracks active MCU sessions.
type Manager struct {
	mu         sync.Mutex
	sessions   map[string]*Session
	connecting map[string]bool
}

var defaultManager = &Manager{
	sessions:   make(map[string]*Session),
	connecting: make(map[string]bool),
}

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
// When baud is 0, common Klipper baud rates are tried in order.
func (m *Manager) Connect(port string, baud int, timeout time.Duration, build cfgtranslate.BuildResult) (*Session, error) {
	if existing, ok := m.GetByPort(port); ok {
		return existing, nil
	}

	m.mu.Lock()
	if m.connecting[port] {
		m.mu.Unlock()
		return nil, fmt.Errorf("connect already in progress for %s — wait for the current attempt to finish", port)
	}
	m.connecting[port] = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.connecting, port)
		m.mu.Unlock()
	}()

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

	lines := cfgtranslate.FinalizeCommands(commands)

	slog.Info("mcu connect starting",
		"port", port,
		"baud", baud,
		"timeout", timeout,
		"objects", len(build.Objects),
		"config_lines", len(lines),
		"init_lines", len(build.InitCommands),
	)

	var conn *Conn
	var usedBaud int
	var err error
	if baud != 0 {
		conn, err = connectDirect(port, baud, timeout, lines, build.InitCommands)
		usedBaud = baud
	} else {
		conn, usedBaud, err = openAndConfigure(port, baud, timeout, lines, build.InitCommands)
	}
	if err != nil {
		slog.Warn("mcu connect failed", "port", port, "error", err)
		return nil, err
	}

	slog.Info("mcu connect syncing clock", "port", port, "baud", usedBaud)
	if err := conn.SyncClock(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("sync clock: %w", err)
	}

	id, err := newSessionID()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	session := NewSession(id, port, usedBaud, conn, build.Objects)
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

// connectDirect opens the port once and configures — matches the pre-servo working path.
func connectDirect(port string, baud int, timeout time.Duration, lines, initCommands []string) (*Conn, error) {
	conn, err := OpenConn(port, baud, timeout)
	if err != nil {
		return nil, err
	}
	if err := conn.ConfigureWithInit(lines, initCommands); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func openAndConfigure(port string, primaryBaud int, timeout time.Duration, lines, initCommands []string) (*Conn, int, error) {
	bauds := baudCandidates(primaryBaud)
	scanning := primaryBaud == 0
	var lastErr error
	for i, baud := range bauds {
		attemptTimeout := timeout
		scanIdentify := false
		if scanning {
			if i < len(bauds)-1 {
				attemptTimeout = 3 * time.Second
				scanIdentify = true
			}
		}
		slog.Info("mcu connect baud attempt",
			"port", port,
			"baud", baud,
			"attempt", i+1,
			"of", len(bauds),
			"timeout", attemptTimeout,
			"scan_identify", scanIdentify,
		)
		start := time.Now()
		conn, err := openAndConfigureBaud(port, baud, attemptTimeout, scanIdentify, lines, initCommands)
		if err == nil {
			slog.Info("mcu connect baud succeeded",
				"port", port,
				"baud", baud,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return conn, baud, nil
		}
		slog.Warn("mcu connect baud failed",
			"port", port,
			"baud", baud,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		lastErr = err
		if primaryBaud != 0 || i == len(bauds)-1 {
			break
		}
	}
	return nil, 0, lastErr
}

func baudCandidates(primaryBaud int) []int {
	if primaryBaud != 0 {
		return []int{primaryBaud}
	}
	// Klipper MCU firmware defaults to 250000; try that first, then common fallbacks.
	return []int{250000, 115200, 230400, 57600}
}

func openAndConfigureBaud(port string, baud int, timeout time.Duration, scanIdentify bool, lines, initCommands []string) (*Conn, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		hardReset := attempt > 0
		slog.Info("mcu serial open",
			"port", port,
			"baud", baud,
			"attempt", attempt+1,
			"hard_reset", hardReset,
		)
		conn, err := openConnWithRetry(port, baud, timeout, hardReset)
		if err != nil {
			slog.Warn("mcu serial open failed", "port", port, "baud", baud, "error", err)
			return nil, err
		}
		if scanIdentify {
			conn.identifyMaxWait = identifyScanWait
		}
		err = conn.ConfigureWithInit(lines, initCommands)
		if err == nil {
			return conn, nil
		}
		slog.Warn("mcu configure attempt failed",
			"port", port,
			"baud", baud,
			"attempt", attempt+1,
			"error", err,
		)
		_ = conn.Close()
		time.Sleep(400 * time.Millisecond)
		lastErr = err
		if attempt == 0 && isRetriableConfigure(err) {
			continue
		}
		break
	}
	if isRetriableConfigure(lastErr) {
		return nil, fmt.Errorf("%w — press reset on the board and try again", lastErr)
	}
	return nil, lastErr
}

func isRetriableConfigure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "shutdown") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "identify")
}
