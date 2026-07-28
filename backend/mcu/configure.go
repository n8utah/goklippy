package mcu

import (
	"fmt"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
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

	for _, command := range commands {
		if c.pinResolver != nil {
			command = c.pinResolver.UpdateCommand(command)
		}
		cmd := c.parser.Create_command(command)
		if len(cmd) == 0 {
			return fmt.Errorf("failed to encode command %q", command)
		}
		block := c.parser.Encode_msgblock(c.nextSeq(), cmd)
		if err := c.writeBlockLocked(block); err != nil {
			return err
		}
	}
	return c.drainLocked(50 * time.Millisecond)
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
	if err := c.Identify(); err != nil {
		return fmt.Errorf("identify: %w", err)
	}

	for _, line := range finalized {
		if err := c.SendCommand(line); err != nil {
			return fmt.Errorf("send %q: %w", line, err)
		}
	}

	deadline := time.Now().Add(c.timeout * 3)
	for time.Now().Before(deadline) {
		status, err := c.GetConfig()
		if err != nil {
			return err
		}
		if status.IsShutdown {
			return fmt.Errorf("mcu shutdown during configure")
		}
		if status.IsConfig {
			break
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

	for _, line := range initCommands {
		if err := c.SendCommand(line); err != nil {
			return fmt.Errorf("init %q: %w", line, err)
		}
	}
	return nil
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
