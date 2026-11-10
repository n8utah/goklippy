// Package sorter is the LEGO sorter's control loop: settings, the hardware it drives,
// and the machine lifecycle. See docs/lego-sorter-server-plan.md.
package sorter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Settings documents. "machine" holds the mapping, gearing and speeds; "routines" holds
// the bench page's rotor routines.
const (
	SettingsMachine  = "machine"
	SettingsRoutines = "routines"
)

// SettingsStore persists named JSON settings documents.
type SettingsStore interface {
	GetSorterSettings(ctx context.Context, name string) (json.RawMessage, bool, error)
	PutSorterSettings(ctx context.Context, name string, data json.RawMessage) error
}

// MachineSettings is the part of the "machine" document the backend reads. The page
// may keep more fields in the document; they are stored untouched.
type MachineSettings struct {
	FeederRpm    map[string]float64         `json:"feederRpm"`
	IndexDegrees float64                    `json:"indexDegrees"`
	IndexRpm     float64                    `json:"indexRpm"`
	IndexAccel   float64                    `json:"indexAccel"` // rpm per second; 0 = no ramp
	DwellMs      int                        `json:"dwellMs"`
	Distributor  DistributorSettings        `json:"distributor"`
	Channels     map[string]ChannelSettings `json:"channels"`
}

// ChannelSettings maps one rotating channel to its stepper.
type ChannelSettings struct {
	Stepper string  `json:"stepper"`
	Forward bool    `json:"forward"`
	Gear    float64 `json:"gear"` // motor turns per rotor turn
}

// DistributorSettings is the chute stepper, its home switch and aiming geometry (see
// chute.go), and the layer servos.
type DistributorSettings struct {
	Stepper string  `json:"stepper"`
	Gear    float64 `json:"gear"`
	Rpm     float64 `json:"rpm"`
	Accel   float64 `json:"accel"` // rpm per second; 0 = no ramp
	// Forward is true when the motor's forward direction turns the chute clockwise
	// seen from above (away from home, toward the bins).
	Forward      bool    `json:"forward"`
	Endstop      string  `json:"endstop"` // the home switch's endstop object
	HomeRpm      float64 `json:"homeRpm"`
	Sections     int     `json:"sections"`
	SectionWidth float64 `json:"sectionWidth"`       // usable degrees of one section
	FirstOffset  float64 `json:"firstSectionOffset"` // home to section 1's start
	MaxAngle     float64 `json:"maxAngle"`           // the chute's clockwise travel limit
	DoorSettleMs int     `json:"doorSettleMs"`       // wait after moving the layer servos

	Layers []LayerSettings `json:"layers"`
}

// LayerSettings is one distribution layer's servo. The selected layer's servo goes to
// OpenAngle and the others to ClosedAngle.
type LayerSettings struct {
	Servo       string  `json:"servo"`
	Bins        int     `json:"bins"`
	OpenAngle   float64 `json:"openAngle"`
	ClosedAngle float64 `json:"closedAngle"`
}

// DefaultMachineSettings matches the bench page's defaults.
func DefaultMachineSettings() MachineSettings {
	return MachineSettings{
		FeederRpm:    map[string]float64{"c1": 5, "c2": 7.5, "c3": 11.25},
		IndexDegrees: 72,
		IndexRpm:     10,
		IndexAccel:   60,
		DwellMs:      1000,
		Distributor: DistributorSettings{
			Stepper: "stepper_e1",
			Gear:    5,
			Rpm:     5,
			Accel:   30,
			Forward: true,
			Endstop: "stepper_e1_endstop",
			HomeRpm: 2,
			// Sorter V2's defaults: 6 sections, 8.25° pillars, first section 8.25°
			// past home. Calibrate on the machine to replace them.
			Sections:     6,
			SectionWidth: 360.0/6 - 8.25,
			FirstOffset:  8.25,
			MaxAngle:     350,
			DoorSettleMs: 300,
			Layers: []LayerSettings{
				{Bins: 18, OpenAngle: 90},
				{Bins: 18, OpenAngle: 90},
				{Bins: 18, OpenAngle: 90},
				{Bins: 12, OpenAngle: 90},
			},
		},
		Channels: map[string]ChannelSettings{
			"c1": {Stepper: "stepper_x", Forward: true, Gear: 1},
			"c2": {Stepper: "stepper_y", Forward: true, Gear: 1},
			"c3": {Stepper: "stepper_z", Forward: true, Gear: 1},
			"c4": {Stepper: "stepper_e0", Forward: true, Gear: 10.8},
		},
	}
}

// parseMachineSettings reads a saved document over the defaults, so fields it lacks
// keep their default values.
func parseMachineSettings(data json.RawMessage) (MachineSettings, error) {
	settings := DefaultMachineSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultMachineSettings(), fmt.Errorf("machine settings: %w", err)
	}
	return settings, nil
}

// SettingsService loads and saves settings documents and keeps the machine settings
// the hardware layer reads.
type SettingsService struct {
	store SettingsStore

	mu      sync.RWMutex
	machine MachineSettings
}

// NewSettingsService loads the saved machine settings. If they can't be read the
// service starts on defaults and logs why, so the backend still runs.
func NewSettingsService(ctx context.Context, store SettingsStore) *SettingsService {
	s := &SettingsService{store: store, machine: DefaultMachineSettings()}
	data, ok, err := store.GetSorterSettings(ctx, SettingsMachine)
	switch {
	case err != nil:
		slog.Warn("sorter settings not loaded, using defaults", "error", err)
	case ok:
		machine, err := parseMachineSettings(data)
		if err != nil {
			slog.Warn("saved sorter settings unreadable, using defaults", "error", err)
		} else {
			s.machine = machine
		}
	}
	return s
}

// Machine returns the current machine settings.
func (s *SettingsService) Machine() MachineSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.machine
}

// Get returns the saved document, and false if none has been saved yet.
func (s *SettingsService) Get(ctx context.Context, name string) (json.RawMessage, bool, error) {
	if err := checkSettingsName(name); err != nil {
		return nil, false, err
	}
	return s.store.GetSorterSettings(ctx, name)
}

// Put validates and saves a document. The machine document must be a JSON object the
// backend can read; routines must be a JSON array.
func (s *SettingsService) Put(ctx context.Context, name string, data json.RawMessage) error {
	if err := checkSettingsName(name); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(data)
	var machine MachineSettings
	switch name {
	case SettingsMachine:
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return fmt.Errorf("%w: machine settings must be a JSON object", ErrInvalidSettings)
		}
		parsed, err := parseMachineSettings(trimmed)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSettings, err)
		}
		machine = parsed
	case SettingsRoutines:
		if len(trimmed) == 0 || trimmed[0] != '[' || !json.Valid(trimmed) {
			return fmt.Errorf("%w: routines must be a JSON array", ErrInvalidSettings)
		}
	}
	if err := s.store.PutSorterSettings(ctx, name, json.RawMessage(trimmed)); err != nil {
		return err
	}
	if name == SettingsMachine {
		s.mu.Lock()
		s.machine = machine
		s.mu.Unlock()
	}
	return nil
}

func checkSettingsName(name string) error {
	if name != SettingsMachine && name != SettingsRoutines {
		return fmt.Errorf("%w: unknown settings %q", ErrInvalidSettings, name)
	}
	return nil
}
