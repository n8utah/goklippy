package mcu

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"goklippy/backend/mcu/msgproto"
	"goklippy/backend/mcu/pins"

	"go.bug.st/serial"
)

const defaultIdentifyChunk = 40

// serialReadTimeout bounds how long a read holds the connection lock when the MCU has
// nothing to say. Motion commands wait on that lock, so it has to stay short.
const serialReadTimeout = 5 * time.Millisecond

// sentBlock is a block the MCU has not acknowledged yet.
type sentBlock struct {
	seq  int
	size int
}

const (
	// maxInflightBytes stays under the MCU's 192-byte receive buffer. Writing past it
	// overflows the buffer, and the MCU then discards everything until resynced.
	maxInflightBytes = 150
	// maxInflightBlocks keeps sequence numbers (4 bits) unambiguous.
	maxInflightBlocks = 12
	// ackWait is how long to wait for room in the window. The MCU drops replies when
	// its transmit buffer is full, so a missing ack doesn't mean the block was lost.
	ackWait = 100 * time.Millisecond
)

// Conn is a Klipper MCU serial connection with message framing.
type Conn struct {
	mu               sync.Mutex
	port             serial.Port
	logPort          string // serial port name (debug logging only)
	parser           *msgproto.MessageParser
	pinResolver      *pins.Resolver
	seq              int
	readBuf          []int
	timeout          time.Duration
	identifyMaxWait  time.Duration // when >0, caps identify wait (baud scan)
	skipConfigSend   bool          // set when MCU already has matching config CRC
	sawIncomingBytes bool
	mcuExpectedSeq   int // next sequence the MCU says it wants, from its latest frame
	shutdownReason   string // set when the MCU reports a shutdown, until it is cleared
	inflight         []sentBlock // blocks written but not yet acknowledged, oldest first
	clockBase        uint64
	clockBaseAt      time.Time
	// hook sees every message read, whichever call reads it (see SetMessageHook).
	hook func(map[string]interface{})
}

// SetMessageHook calls fn with every message read from the MCU, so reports it sends on
// its own (analog readings) aren't lost when a query reads past them. fn runs with the
// connection locked: it must be quick and must not call back into the connection.
func (c *Conn) SetMessageHook(fn func(map[string]interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hook = fn
}

// OpenConn opens a serial port and prepares for Klipper protocol communication.
func OpenConn(portName string, baud int, timeout time.Duration) (*Conn, error) {
	return openConnWithRetry(portName, baud, timeout, false)
}

func openConnWithRetry(portName string, baud int, timeout time.Duration, hardReset bool) (*Conn, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		conn, err := openConn(portName, baud, timeout, hardReset)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if !isPortBusyErr(err) || attempt == 3 {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * 400 * time.Millisecond)
	}
	return nil, lastErr
}

func isPortBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "in use") || strings.Contains(msg, "busy")
}

func openConn(portName string, baud int, timeout time.Duration, hardReset bool) (*Conn, error) {
	if baud == 0 {
		baud = 250000
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, friendlyOpenError(portName, err)
	}

	if err := port.SetReadTimeout(serialReadTimeout); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("set read timeout: %w", err)
	}

	if hardReset {
		pulseDTRReset(port)
	}

	waitForBoot(hardReset)
	// Only drain stale bytes after an explicit reset; draining on every open was
	// discarding data and the shorter boot wait caused identify timeouts.
	if hardReset {
		drainSerialPort(port)
	}

	return &Conn{
		port:    port,
		logPort: portName,
		parser:  msgproto.NewMessageParser("mcu: "),
		seq:     0,
		timeout: timeout,
	}, nil
}

// pulseDTRReset toggles DTR to reset Arduino-class boards (RAMPS/ATmega2560).
func pulseDTRReset(port serial.Port) {
	_ = port.SetDTR(false)
	time.Sleep(100 * time.Millisecond)
	_ = port.SetDTR(true)
	time.Sleep(100 * time.Millisecond)
	_ = port.SetDTR(false)
	time.Sleep(100 * time.Millisecond)
}

// waitForBoot pauses after port open so the MCU can finish resetting and boot Klipper.
func waitForBoot(afterReset bool) {
	if afterReset {
		time.Sleep(4 * time.Second)
		return
	}
	// Opening the serial port often toggles DTR and resets Arduino-class boards.
	time.Sleep(2 * time.Second)
}

func drainSerialPort(port serial.Port) {
	buf := make([]byte, 256)
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, err := port.Read(buf)
		if err != nil || n == 0 {
			time.Sleep(15 * time.Millisecond)
		}
	}
}

func friendlyOpenError(portName string, err error) error {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "being used") ||
		strings.Contains(msg, "busy") ||
		strings.Contains(msg, "permission") {
		return fmt.Errorf(
			"port %s is in use — disconnect in goklippy, close other apps (Cura, Printrun, serial monitor), or press reset on the board: %w",
			portName, err,
		)
	}
	return fmt.Errorf("open serial port %s: %w", portName, err)
}

// Reopen closes and reopens the port at baud, which resets Arduino-class boards, and
// clears all protocol state. Call Identify or ConfigureWithInit afterwards.
func (c *Conn) Reopen(baud int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.port == nil {
		return fmt.Errorf("serial port closed")
	}
	if err := c.port.Close(); err != nil {
		slog.Warn("closing serial port for reset", "port", c.logPort, "error", err)
	}
	c.port = nil

	fresh, err := openConnWithRetry(c.logPort, baud, c.timeout, false)
	if err != nil {
		return err
	}
	c.port = fresh.port
	c.parser = fresh.parser
	c.pinResolver = nil
	c.readBuf = nil
	c.seq = 0
	c.inflight = nil
	c.sawIncomingBytes = false
	c.skipConfigSend = false
	c.shutdownReason = ""
	return nil
}

// Close closes the serial port.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.port == nil {
		return nil
	}
	err := c.port.Close()
	c.port = nil
	return err
}

// Parser returns the message parser (populated after identify).
func (c *Conn) Parser() *msgproto.MessageParser {
	return c.parser
}

// SyncClock queries get_clock and stores the baseline.
func (c *Conn) SyncClock() error {
	params, err := c.QueryCommand("get_clock", "clock")
	if err != nil {
		return err
	}
	clock, ok := params["clock"].(int64)
	if !ok {
		return fmt.Errorf("get_clock: unexpected response")
	}
	c.mu.Lock()
	c.clockBase = uint64(clock)
	c.clockBaseAt = time.Now()
	c.mu.Unlock()
	return nil
}

// Identify fetches the MCU data dictionary and initializes the message parser.
func (c *Conn) Identify() error {
	slog.Info("mcu identify starting",
		"port", c.logPort,
		"timeout", c.timeout,
		"identify_wait", c.effectiveIdentifyWait(),
	)
	start := time.Now()
	data, err := c.fetchIdentifyData()
	if err != nil {
		slog.Warn("mcu identify failed",
			"port", c.logPort,
			"duration_ms", time.Since(start).Milliseconds(),
			"saw_bytes", c.sawIncomingBytes,
			"error", err,
		)
		return err
	}
	c.parser.Process_identify(data, true)
	c.pinResolver = pins.NewArduinoMegaResolver()
	slog.Info("mcu identify complete",
		"port", c.logPort,
		"dictionary_bytes", len(data),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (c *Conn) fetchIdentifyData() ([]byte, error) {
	identifyData := make([]byte, 0, 4096)
	wait := c.effectiveIdentifyWait()
	deadline := time.Now().Add(wait)
	lastProgress := time.Now()

	for time.Now().Before(deadline) {
		if time.Since(lastProgress) >= 3*time.Second {
			slog.Info("mcu identify waiting",
				"port", c.logPort,
				"bytes_received", len(identifyData),
				"saw_incoming", c.sawIncomingBytes,
				"remaining_ms", time.Until(deadline).Milliseconds(),
			)
			lastProgress = time.Now()
		}

		msg := fmt.Sprintf("identify offset=%d count=%d", len(identifyData), defaultIdentifyChunk)
		params, err := c.sendWithResponseUntil(msg, "identify_response", deadline)
		if err != nil {
			if isTimeoutErr(err) {
				continue
			}
			return nil, err
		}

		offset, ok := params["offset"].(int64)
		if !ok {
			return nil, fmt.Errorf("identify_response missing offset")
		}
		if int(offset) != len(identifyData) {
			continue
		}

		rawData, hasData := params["data"]
		if !hasData || rawData == nil {
			return identifyData, nil
		}

		chunk, ok := rawData.([]int)
		if !ok {
			return nil, fmt.Errorf("identify_response data has unexpected type %T", rawData)
		}
		if len(chunk) == 0 {
			return identifyData, nil
		}

		for _, b := range chunk {
			identifyData = append(identifyData, byte(b))
		}
	}

	if c.sawIncomingBytes && len(c.readBuf) > 0 {
		logIdentifyDebug(c.readBuf)
		return nil, fmt.Errorf(
			"identify timed out after %s — data on serial but no Klipper identify_response (wrong baud, Marlin instead of Klipper firmware, or noisy USB — verify Klipper is flashed at 250000 or 115200)",
			wait,
		)
	}
	if c.sawIncomingBytes {
		return nil, fmt.Errorf(
			"identify timed out after %s — serial activity detected but no valid Klipper frames (try pressing reset on the board)",
			wait,
		)
	}
	return nil, fmt.Errorf("identify timed out after %s — no response on serial port (check USB cable, COM port, and that Klipper firmware is flashed)", wait)
}

func (c *Conn) effectiveIdentifyWait() time.Duration {
	wait := identifyWait(c.timeout)
	if c.identifyMaxWait > 0 && c.identifyMaxWait < wait {
		return c.identifyMaxWait
	}
	return wait
}

func identifyWait(opTimeout time.Duration) time.Duration {
	if opTimeout <= 0 {
		opTimeout = 5 * time.Second
	}
	wait := opTimeout * 4
	if wait < 8*time.Second {
		return 8 * time.Second
	}
	if wait > 20*time.Second {
		return 20 * time.Second
	}
	return wait
}

func logIdentifyDebug(buf []int) {
	const max = 48
	n := len(buf)
	if n > max {
		n = max
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("%02x", buf[i]&0xff)
	}
	slog.Warn("identify failed with serial data but no valid Klipper frames",
		"bytes_buffered", len(buf),
		"sample_hex", strings.Join(parts, " "),
	)
}

func isTimeoutErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timeout waiting for")
}

func (c *Conn) sendWithResponse(command, responseName string) (map[string]interface{}, error) {
	return c.sendWithResponseUntil(command, responseName, time.Now().Add(c.timeout))
}

func (c *Conn) sendWithResponseUntil(command, responseName string, deadline time.Time) (map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queryLocked(command, responseName, deadline)
}

func (c *Conn) queryLocked(command, responseName string, deadline time.Time) (map[string]interface{}, error) {
	if err := c.sendCommandsLocked([]string{command}); err != nil {
		return nil, err
	}

	for time.Now().Before(deadline) {
		params, err := c.readMessageLocked()
		if err != nil {
			return nil, err
		}
		if params == nil {
			continue
		}
		name, _ := params["#name"].(string)
		if name == responseName {
			return params, nil
		}
	}

	return nil, fmt.Errorf("timeout waiting for %q", responseName)
}

// ackThrough drops blocks the MCU has consumed; expected is the next one it wants.
func (c *Conn) ackThrough(expected int) {
	for i, b := range c.inflight {
		if b.seq == expected {
			c.inflight = c.inflight[i:]
			return
		}
	}
	c.inflight = c.inflight[:0]
}

func inflightBytes(blocks []sentBlock) int {
	total := 0
	for _, b := range blocks {
		total += b.size
	}
	return total
}

// waitForWindow blocks until a block of size bytes fits in the MCU's receive buffer.
func (c *Conn) waitForWindow(size int) error {
	deadline := time.Now().Add(ackWait)
	for len(c.inflight) >= maxInflightBlocks || (len(c.inflight) > 0 && inflightBytes(c.inflight)+size > maxInflightBytes) {
		if time.Now().After(deadline) {
			slog.Warn("mcu ack overdue, continuing", "port", c.logPort, "unacked_blocks", len(c.inflight))
			c.inflight = c.inflight[:0]
			return nil
		}
		if _, err := c.readMessageLocked(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) nextSeq() int {
	seq := c.seq & msgproto.MESSAGE_SEQ_MASK
	c.seq = (c.seq + 1) & msgproto.MESSAGE_SEQ_MASK
	return seq
}

func (c *Conn) writeBlockLocked(block []int) error {
	buf := make([]byte, len(block))
	for i, v := range block {
		buf[i] = byte(v)
	}
	n, err := c.port.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("short write: %d/%d bytes", n, len(buf))
	}
	return nil
}

func (c *Conn) recentIncomingBytes() bool {
	return c.sawIncomingBytes
}

// drainIdle ends a drain once the MCU has been quiet this long.
const drainIdle = 8 * time.Millisecond

// DrainInbound reads messages until idle for drainFor, invoking handler for each.
func (c *Conn) DrainInbound(drainFor time.Duration, handler func(map[string]interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idleDeadline := time.Now().Add(drainIdle)
	deadline := time.Now().Add(drainFor)
	for time.Now().Before(deadline) {
		params, err := c.readMessageLocked()
		if err != nil {
			return
		}
		if params == nil {
			if time.Now().After(idleDeadline) {
				return
			}
			continue
		}
		idleDeadline = time.Now().Add(drainIdle)
		if handler != nil {
			handler(params)
		}
	}
}

// logIfShutdown surfaces MCU shutdown reports, which would otherwise be discarded
// by whichever call happened to be reading.
func (c *Conn) logIfShutdown(params map[string]interface{}) {
	name, _ := params["#name"].(string)
	if name != "shutdown" && name != "is_shutdown" {
		return
	}
	reason := fmt.Sprint(params["static_string_id"])
	if c.shutdownReason != reason {
		slog.Warn("mcu reported shutdown", "port", c.logPort, "reason", reason)
	}
	c.shutdownReason = reason
}

// ShutdownReason returns why the MCU shut down, or "" if it has not. It only reflects
// frames already read, so use GetConfig when the answer has to be current.
func (c *Conn) ShutdownReason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shutdownReason
}

// SeqState returns the next sequence the host will send and the one the MCU last
// asked for. Once traffic settles they should match; if not, the MCU is discarding
// everything the host sends.
func (c *Conn) SeqState() (hostNext, mcuExpected int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq, c.mcuExpectedSeq
}

func (c *Conn) readMessageLocked() (map[string]interface{}, error) {
	for {
		if msgLen := c.parser.Check_packet(c.readBuf); msgLen > 0 {
			frame := c.readBuf[:msgLen]
			c.readBuf = c.readBuf[msgLen:]
			c.mcuExpectedSeq = frame[msgproto.MESSAGE_POS_SEQ] & msgproto.MESSAGE_SEQ_MASK
			c.ackThrough(c.mcuExpectedSeq)
			if params, ok := c.parser.ParseSafe(frame); ok {
				c.logIfShutdown(params)
				if c.hook != nil {
					c.hook(params)
				}
				return params, nil
			}
			continue
		}
		if msgLen := c.parser.Check_packet(c.readBuf); msgLen < 0 {
			c.readBuf = c.readBuf[1:]
			continue
		}

		tmp := make([]byte, 128)
		n, err := c.port.Read(tmp)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("serial port closed")
			}
			return nil, fmt.Errorf("serial read: %w", err)
		}
		if n == 0 {
			return nil, nil
		}
		c.sawIncomingBytes = true
		for i := 0; i < n; i++ {
			c.readBuf = append(c.readBuf, int(tmp[i]))
		}
	}
}
