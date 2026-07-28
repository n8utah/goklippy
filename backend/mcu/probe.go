package mcu

import (
	"fmt"
	"time"
)

// IdentifyResult holds MCU information from the identify handshake.
type IdentifyResult struct {
	Port          string
	Baud          int
	Version       string
	BuildVersions string
	MessageCount  int
	Constants     map[string]interface{}
}

// Probe opens a port, runs identify, and returns MCU metadata.
func Probe(portName string, baud int, timeout time.Duration) (IdentifyResult, error) {
	conn, err := OpenConn(portName, baud, timeout)
	if err != nil {
		return IdentifyResult{}, err
	}
	defer conn.Close()

	if err := conn.Identify(); err != nil {
		if conn.recentIncomingBytes() {
			return IdentifyResult{}, fmt.Errorf(
				"%w: MCU is sending data on %s but did not reply to identify — close other apps using the port, press reset on the board, try another USB data cable, then retry",
				err, portName,
			)
		}
		return IdentifyResult{}, err
	}

	parser := conn.Parser()
	version, build := parser.Get_version_info()

	return IdentifyResult{
		Port:          portName,
		Baud:          baud,
		Version:       version,
		BuildVersions: build,
		MessageCount:  parser.MessageCount(),
		Constants:     parser.Get_constants(),
	}, nil
}

// String formats a human-readable summary.
func (r IdentifyResult) String() string {
	return fmt.Sprintf(
		"port=%s baud=%d version=%q build=%q messages=%d",
		r.Port, r.Baud, r.Version, r.BuildVersions, r.MessageCount,
	)
}

// DefaultProbeTimeout is used when no timeout is configured.
const DefaultProbeTimeout = 5 * time.Second
