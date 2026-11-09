package discovery

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"goklippy/backend/mcu"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// PortInfo describes a serial port and optional Klipper probe result.
type PortInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsUSB       bool   `json:"is_usb"`
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

// enumerate lists the OS serial ports, with USB details where available.
func enumerate() ([]PortInfo, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		slog.Warn("detailed serial port enumeration failed; falling back to names", "error", err)
		names, listErr := serial.GetPortsList()
		if listErr != nil {
			return nil, listErr
		}
		out := make([]PortInfo, 0, len(names))
		for _, name := range names {
			out = append(out, PortInfo{Name: name})
		}
		return out, nil
	}

	out := make([]PortInfo, 0, len(details))
	for _, d := range details {
		out = append(out, PortInfo{Name: d.Name, Description: describePort(d), IsUSB: d.IsUSB})
	}
	return out, nil
}

func describePort(d *enumerator.PortDetails) string {
	if d.Product != "" {
		return d.Product
	}
	if d.IsUSB {
		return fmt.Sprintf("USB %s:%s", d.VID, d.PID)
	}
	return ""
}

// ListPorts returns the ports attached right now without opening them. Results
// from the last scan are carried over for ports that are still present.
func ListPorts() ([]PortInfo, error) {
	ports, err := enumerate()
	if err != nil {
		return nil, err
	}
	if cached, ok := Cached(); ok {
		mergeProbeResults(ports, cached.Ports)
	}
	return ports, nil
}

func mergeProbeResults(ports, probed []PortInfo) {
	byName := make(map[string]PortInfo, len(probed))
	for _, p := range probed {
		byName[p.Name] = p
	}
	for i := range ports {
		if p, ok := byName[ports[i].Name]; ok {
			ports[i].IsKlipper = p.IsKlipper
			ports[i].Version = p.Version
			ports[i].Build = p.Build
			ports[i].MessageCount = p.MessageCount
			ports[i].Error = p.Error
		}
	}
}

// Scan enumerates ports and probes each for Klipper identify.
func Scan(baud int, probeTimeout time.Duration) (ScanResult, error) {
	if baud == 0 {
		baud = 250000
	}
	if probeTimeout == 0 {
		probeTimeout = 2 * time.Second
	}

	ports, err := enumerate()
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{
		ScannedAt: time.Now().UTC(),
		Ports:     make([]PortInfo, 0, len(ports)),
	}

	for _, info := range ports {
		name := info.Name

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
