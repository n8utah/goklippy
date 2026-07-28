package mcu

import (
	"fmt"
	"io"
	"sync"
	"time"

	"goklippy/backend/mcu/msgproto"
	"goklippy/backend/mcu/pins"

	"go.bug.st/serial"
)

const defaultIdentifyChunk = 40

// Conn is a Klipper MCU serial connection with message framing.
type Conn struct {
	mu               sync.Mutex
	port             serial.Port
	parser           *msgproto.MessageParser
	pinResolver      *pins.Resolver
	seq              int
	readBuf          []int
	timeout          time.Duration
	sawIncomingBytes bool
	clockBase        uint64
	clockBaseAt      time.Time
}

// OpenConn opens a serial port and prepares for Klipper protocol communication.
func OpenConn(portName string, baud int, timeout time.Duration) (*Conn, error) {
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
		return nil, fmt.Errorf("open serial port %s: %w", portName, err)
	}

	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("set read timeout: %w", err)
	}

	waitForBoot()

	return &Conn{
		port:    port,
		parser:  msgproto.NewMessageParser("mcu: "),
		seq:     0,
		timeout: timeout,
	}, nil
}

// waitForBoot pauses after port open so the MCU can finish resetting and boot Klipper.
func waitForBoot() {
	time.Sleep(2 * time.Second)
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
	data, err := c.fetchIdentifyData()
	if err != nil {
		return err
	}
	c.parser.Process_identify(data, true)
	c.pinResolver = pins.NewArduinoMegaResolver()
	return nil
}

func (c *Conn) fetchIdentifyData() ([]byte, error) {
	identifyData := make([]byte, 0, 4096)
	deadline := time.Now().Add(c.timeout * 4)

	for time.Now().Before(deadline) {
		msg := fmt.Sprintf("identify offset=%d count=%d", len(identifyData), defaultIdentifyChunk)
		params, err := c.sendWithResponse(msg, "identify_response")
		if err != nil {
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

	return nil, fmt.Errorf("identify timed out after %s", c.timeout*4)
}

func (c *Conn) sendWithResponse(command, responseName string) (map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pinResolver != nil {
		command = c.pinResolver.UpdateCommand(command)
	}

	cmd := c.parser.Create_command(command)
	if len(cmd) == 0 {
		return nil, fmt.Errorf("failed to encode command %q", command)
	}

	block := c.parser.Encode_msgblock(c.nextSeq(), cmd)
	if err := c.writeBlockLocked(block); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(c.timeout)
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
	return c.port.Drain()
}

func (c *Conn) recentIncomingBytes() bool {
	return c.sawIncomingBytes
}

// DrainInbound reads messages until idle for drainFor, invoking handler for each.
func (c *Conn) DrainInbound(drainFor time.Duration, handler func(map[string]interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idleDeadline := time.Now().Add(30 * time.Millisecond)
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
		idleDeadline = time.Now().Add(30 * time.Millisecond)
		if handler != nil {
			handler(params)
		}
	}
}

func (c *Conn) readMessageLocked() (map[string]interface{}, error) {
	for {
		if msgLen := c.parser.Check_packet(c.readBuf); msgLen > 0 {
			frame := c.readBuf[:msgLen]
			c.readBuf = c.readBuf[msgLen:]
			if params, ok := c.parser.ParseSafe(frame); ok {
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
