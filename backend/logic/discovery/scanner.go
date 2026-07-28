package discovery

import (
	"fmt"
	"sync"
	"time"

	"goklippy/backend/mcu"

	"go.bug.st/serial"
)

// PortInfo describes a serial port and optional Klipper probe result.
type PortInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsKlipper   bool   `json:"is_klipper"`
	Version     string `json:"version,omitempty"`
	Build       string `json:"build,omitempty"`
	MessageCount int   `json:"message_count,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ScanResult is the cached output of a port scan.
type ScanResult struct {
	ScannedAt time.Time  `json:"scanned_at"`
	Ports     []PortInfo `json:"ports"`
}

var (
	cacheMu sync.RWMutex
	cache   ScanResult
)

// ListPorts returns OS serial ports without probing.
func ListPorts() ([]PortInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]PortInfo, 0, len(ports))
	for _, name := range ports {
		out = append(out, PortInfo{Name: name})
	}
	return out, nil
}

// Scan enumerates ports and probes each for Klipper identify.
func Scan(baud int, probeTimeout time.Duration) (ScanResult, error) {
	if baud == 0 {
		baud = 250000
	}
	if probeTimeout == 0 {
		probeTimeout = 2 * time.Second
	}

	names, err := serial.GetPortsList()
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{
		ScannedAt: time.Now().UTC(),
		Ports:     make([]PortInfo, 0, len(names)),
	}

	for _, name := range names {
		info := PortInfo{Name: name}

		if session, ok := mcu.DefaultManager().GetByPort(name); ok {
			info.IsKlipper = true
			info.Description = "connected"
			if v, ok := session.Constants["MCU_VERSION"]; ok {
				info.Version = fmt.Sprint(v)
			}
			result.Ports = append(result.Ports, info)
			continue
		}

		probeResult, err := mcu.Probe(name, baud, probeTimeout)
		if err != nil {
			info.Error = err.Error()
		} else {
			info.IsKlipper = true
			info.Version = probeResult.Version
			info.Build = probeResult.BuildVersions
			info.MessageCount = probeResult.MessageCount
		}
		result.Ports = append(result.Ports, info)
	}

	cacheMu.Lock()
	cache = result
	cacheMu.Unlock()
	return result, nil
}

// Cached returns the last scan result, if any.
func Cached() (ScanResult, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if cache.ScannedAt.IsZero() {
		return ScanResult{}, false
	}
	return cache, true
}
