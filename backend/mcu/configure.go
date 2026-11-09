package mcu

import (
	"fmt"
	"log/slog"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
	"goklippy/backend/mcu/msgproto"
)

// ConfigStatus is the parsed get_config response.
type ConfigStatus struct {
	IsConfig   bool
	CRC        uint32
	IsShutdown bool
	MoveCount  int
}

// MCUClock returns the current MCU clock tick (32-bit).
func (c *Conn) MCUClock() (uint32, error) {
	params, err := c.QueryCommand("get_clock", "clock")
	if err != nil {
		return 0, err
	}
	clock, ok := params["clock"].(int64)
	if !ok {
		return 0, fmt.Errorf("get_clock: unexpected response")
	}
	return uint32(clock), nil
}

// SendCommands sends multiple commands under one lock, draining once at the end.
func (c *Conn) SendCommands(commands []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendCommandsLocked(commands)
}

// SendAtClock reads the MCU clock and sends the commands built from it without letting
// any other traffic in between, so a move's start time can't pass while it waits for
// the serial port.
func (c *Conn) SendAtClock(marginTicks uint32, build func(startClock uint32) []string) (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	params, err := c.queryLocked("get_clock", "clock", time.Now().Add(c.timeout))
	if err != nil {
		return 0, err
	}
	clock, ok := params["clock"].(int64)
	if !ok {
		return 0, fmt.Errorf("get_clock: unexpected response")
	}
	startClock := uint32(clock) + marginTicks
	return startClock, c.sendCommandsLocked(build(startClock))
}

func (c *Conn) sendCommandsLocked(commands []string) error {
	for _, command := range commands {
		if c.pinResolver != nil {
			command = c.pinResolver.UpdateCommand(command)
		}
		cmd := c.parser.Create_command(command)
		if len(cmd) == 0 {
			return fmt.Errorf("failed to encode command %q", command)
		}
		if err := c.sendBlockLocked(cmd); err != nil {
			return err
		}
	}
	return nil
}

// sendBlockLocked writes one command once it fits in the MCU's receive buffer.
func (c *Conn) sendBlockLocked(cmd []int) error {
	if err := c.waitForWindow(msgproto.MESSAGE_MIN + len(cmd)); err != nil {
		return err
	}
	seq := c.nextSeq()
	block := c.parser.Encode_msgblock(seq, cmd)
	if err := c.writeBlockLocked(block); err != nil {
		return err
	}
	c.inflight = append(c.inflight, sentBlock{seq: seq, size: len(block)})
	return nil
}

// SendCommand sends a host→MCU command without waiting for a specific response.
func (c *Conn) SendCommand(command string) error {
	return c.SendCommands([]string{command})
}

// QueryCommand sends a command and waits for a named response.
func (c *Conn) QueryCommand(command, responseName string) (map[string]interface{}, error) {
	return c.sendWithResponse(command, responseName)
}

// GetConfig queries MCU configuration state.
func (c *Conn) GetConfig() (ConfigStatus, error) {
	params, err := c.QueryCommand("get_config", "config")
	if err != nil {
		return ConfigStatus{}, err
	}
	status := ConfigStatus{}
	if v, ok := params["is_config"].(int64); ok {
		status.IsConfig = v != 0
	}
	if v, ok := params["crc"].(int64); ok {
		status.CRC = uint32(v)
	}
	if v, ok := params["is_shutdown"].(int64); ok {
		status.IsShutdown = v != 0
	}
	if v, ok := params["move_count"].(int64); ok {
		status.MoveCount = int(v)
	}
	return status, nil
}

// Configure sends config commands and waits until the MCU reports configured.
func (c *Conn) Configure(configLines []string) error {
	return c.ConfigureWithInit(cfgtranslate.FinalizeCommands(configLines), nil)
}

// ConfigureWithInit sends config + init commands after identify.
func (c *Conn) ConfigureWithInit(finalized []string, initCommands []string) error {
	start := time.Now()
	c.skipConfigSend = false
	slog.Info("mcu configure starting",
		"port", c.logPort,
		"config_lines", len(finalized),
		"init_lines", len(initCommands),
	)

	if err := c.Identify(); err != nil {
		return fmt.Errorf("identify: %w", err)
	}
	if err := c.prepareForConfig(finalized); err != nil {
		return err
	}

	if !c.skipConfigSend {
		slog.Info("mcu configure sending config lines", "port", c.logPort, "lines", len(finalized))
		for i, line := range finalized {
			if err := c.SendCommand(line); err != nil {
				return fmt.Errorf("send %q: %w", line, err)
			}
			if (i+1)%10 == 0 || i+1 == len(finalized) {
				slog.Info("mcu configure progress",
					"port", c.logPort,
					"sent", i+1,
					"total", len(finalized),
				)
			}
		}
	} else {
		slog.Info("mcu configure skipped config lines (already configured)", "port", c.logPort)
	}

	deadline := time.Now().Add(configureWait(c.timeout))
	lastWaitLog := time.Now()
	for time.Now().Before(deadline) {
		status, err := c.GetConfig()
		if err != nil {
			return err
		}
		if status.IsShutdown {
			slog.Warn("mcu shutdown during configure",
				"port", c.logPort,
				"is_config", status.IsConfig,
				"crc", status.CRC,
				"move_count", status.MoveCount,
			)
			return fmt.Errorf("mcu shutdown during configure")
		}
		if status.IsConfig {
			break
		}
		if time.Since(lastWaitLog) >= 3*time.Second {
			slog.Info("mcu configure waiting for is_config",
				"port", c.logPort,
				"remaining_ms", time.Until(deadline).Milliseconds(),
			)
			lastWaitLog = time.Now()
		}
		time.Sleep(20 * time.Millisecond)
	}

	status, err := c.GetConfig()
	if err != nil {
		return err
	}
	if !status.IsConfig {
		return fmt.Errorf("configure timed out")
	}

	slog.Info("mcu configure sending init lines", "port", c.logPort, "lines", len(initCommands))
	for _, line := range initCommands {
		if err := c.SendCommand(line); err != nil {
			return fmt.Errorf("init %q: %w", line, err)
		}
	}

	status, err = c.GetConfig()
	if err != nil {
		return err
	}
	if status.IsShutdown {
		slog.Warn("mcu shutdown after init",
			"port", c.logPort,
			"crc", status.CRC,
			"move_count", status.MoveCount,
		)
		return fmt.Errorf("mcu shutdown after init commands")
	}

	slog.Info("mcu configure complete",
		"port", c.logPort,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (c *Conn) prepareForConfig(finalized []string) error {
	status, err := c.GetConfig()
	if err != nil {
		return err
	}

	if status.IsConfig {
		expectedCRC := configCRC(finalized)
		if expectedCRC != 0 && status.CRC == expectedCRC {
			slog.Info("mcu already configured with matching crc",
				"port", c.logPort,
				"crc", status.CRC,
			)
			c.skipConfigSend = true
			return nil
		}
		slog.Warn("mcu already configured, resetting before reconfigure",
			"port", c.logPort,
			"mcu_crc", status.CRC,
			"expected_crc", expectedCRC,
		)
		if err := c.resetForReconfigure(); err != nil {
			return fmt.Errorf("reset mcu before reconfigure: %w", err)
		}
		status, err = c.GetConfig()
		if err != nil {
			return err
		}
	}

	if !status.IsShutdown {
		slog.Info("mcu pre-config status",
			"port", c.logPort,
			"is_shutdown", false,
			"is_config", status.IsConfig,
		)
		return nil
	}
	slog.Warn("mcu is shutdown before configure, clearing", "port", c.logPort)
	if err := c.ClearShutdown(); err != nil {
		return fmt.Errorf("mcu is shutdown: %w", err)
	}
	time.Sleep(50 * time.Millisecond)
	status, err = c.GetConfig()
	if err != nil {
		return err
	}
	if status.IsShutdown {
		return fmt.Errorf("mcu still shutdown after clear_shutdown")
	}
	return nil
}

func configCRC(finalized []string) uint32 {
	for i := len(finalized) - 1; i >= 0; i-- {
		var crc uint64
		if _, err := fmt.Sscanf(finalized[i], "finalize_config crc=%d", &crc); err == nil {
			return uint32(crc)
		}
	}
	return 0
}

// resetForReconfigure pulse-resets the MCU and re-runs identify.
func (c *Conn) resetForReconfigure() error {
	c.mu.Lock()
	port := c.port
	c.mu.Unlock()
	if port == nil {
		return fmt.Errorf("serial port closed")
	}
	pulseDTRReset(port)
	waitForBoot(true)
	drainSerialPort(port)

	c.mu.Lock()
	c.parser = msgproto.NewMessageParser("mcu: ")
	c.pinResolver = nil
	c.readBuf = c.readBuf[:0]
	c.seq = 0
	c.sawIncomingBytes = false
	c.skipConfigSend = false
	c.shutdownReason = ""
	c.inflight = c.inflight[:0]
	c.mu.Unlock()

	return c.Identify()
}

// ClearShutdown clears a Klipper MCU shutdown state so config can proceed.
func (c *Conn) ClearShutdown() error {
	cmd := c.parser.Create_command("clear_shutdown")
	if len(cmd) == 0 {
		return fmt.Errorf("clear_shutdown command not available")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendBlockLocked(cmd); err != nil {
		return err
	}
	// Reports already in flight are consumed first so they can't re-mark the shutdown.
	err := c.drainLocked(200 * time.Millisecond)
	c.shutdownReason = ""
	return err
}

func configureWait(opTimeout time.Duration) time.Duration {
	if opTimeout <= 0 {
		opTimeout = 5 * time.Second
	}
	wait := opTimeout * 6
	if wait < 30*time.Second {
		return 30 * time.Second
	}
	return wait
}

func (c *Conn) drainLocked(max time.Duration) error {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		params, err := c.readMessageLocked()
		if err != nil {
			return err
		}
		if params == nil {
			continue
		}
	}
	return nil
}

// ConfigureFile loads hardware commands from a parsed config via cfgtranslate.
func ConfigureFromLines(port string, baud int, timeout time.Duration, configLines []string) error {
	conn, err := OpenConn(port, baud, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Configure(configLines)
}
