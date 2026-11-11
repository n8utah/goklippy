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

	"goklippy/backend/logic/vision"
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
	Feeder       FeederSettings             `json:"feeder"`
	Platter      PlatterSettings            `json:"platter"`
	Cameras      CameraSettings             `json:"cameras"`
	Lights       LightsSettings             `json:"lights"`
	Pictures     PicturesSettings           `json:"pictures"`
	Supply       SupplySettings             `json:"supply"`
}

// SupplySettings read the motors' 12 V supply through a resistor divider into an
// analog pin: [voltage_sensor supply] with sensor_pin: analog5 in the board config.
type SupplySettings struct {
	// Sensor is the config section; when the board config hasn't got it, the supply
	// isn't measured and nothing is held back.
	Sensor string `json:"sensor"`
	// TopOhms runs from the supply to the pin, BottomOhms from the pin to ground.
	TopOhms    float64 `json:"topOhms"`
	BottomOhms float64 `json:"bottomOhms"`
	// ReferenceV is the ADC's full scale (the Mega's 5 V); Calibration multiplies the
	// result, set from a meter reading.
	ReferenceV  float64 `json:"referenceV"`
	Calibration float64 `json:"calibration"`
	// MinVolts: below this the motors aren't properly powered; homing and sorting are
	// refused, and a run stops.
	MinVolts float64 `json:"minVolts"`
}

// Volts turns a reading (a share of the ADC's range) into the supply's voltage.
func (s SupplySettings) Volts(fraction float64) float64 {
	top, bottom := s.TopOhms, s.BottomOhms
	if bottom <= 0 {
		top, bottom = 0, 1 // no divider
	}
	ref, cal := s.ReferenceV, s.Calibration
	if ref <= 0 {
		ref = 5
	}
	if cal <= 0 {
		cal = 1
	}
	return fraction * ref * (top + bottom) / bottom * cal
}

// PicturesSettings decide which frames of a tracked piece are good pictures of it
// (see journeys.go). 0 uses the default for each.
type PicturesSettings struct {
	// IsolationPx: another piece's box this close makes a frame "not alone".
	IsolationPx int `json:"isolationPx"`
	// SettleFrames: a track's first frames are skipped while it lands and bounces.
	SettleFrames int `json:"settleFrames"`
	// MaxDegPerFrame: faster than this, the piece is moving too much for a sharp
	// picture.
	MaxDegPerFrame float64 `json:"maxDegPerFrame"`
	// MinSharpness: the crop's detail (variance of its Laplacian) must reach this.
	MinSharpness float64 `json:"minSharpness"`
	// SizeFactor: a box more than this factor off the track's usual area means it's
	// touching something or partly hidden.
	SizeFactor float64 `json:"sizeFactor"`
	// KeepEveryDeg: kept pictures are at least this far apart along the channel, so
	// they show the piece turned different ways; the sharpest in each stretch wins.
	KeepEveryDeg float64 `json:"keepEveryDeg"`
	MaxKept      int     `json:"maxKept"` // per channel
	// CropMarginPct pads the crop on each side, as a share of the box; CropMaxPx caps
	// the crop's longer side.
	CropMarginPct float64 `json:"cropMarginPct"`
	CropMaxPx     int     `json:"cropMaxPx"`
	// KeepJourneys is how many finished journeys keep their debug pictures.
	KeepJourneys int `json:"keepJourneys"`
}

// withDefaults fills in the unset fields.
func (p PicturesSettings) withDefaults() PicturesSettings {
	d := DefaultPictures()
	if p.IsolationPx <= 0 {
		p.IsolationPx = d.IsolationPx
	}
	if p.SettleFrames <= 0 {
		p.SettleFrames = d.SettleFrames
	}
	if p.MaxDegPerFrame <= 0 {
		p.MaxDegPerFrame = d.MaxDegPerFrame
	}
	if p.MinSharpness <= 0 {
		p.MinSharpness = d.MinSharpness
	}
	if p.SizeFactor <= 1 {
		p.SizeFactor = d.SizeFactor
	}
	if p.KeepEveryDeg <= 0 {
		p.KeepEveryDeg = d.KeepEveryDeg
	}
	if p.MaxKept <= 0 {
		p.MaxKept = d.MaxKept
	}
	if p.CropMarginPct <= 0 {
		p.CropMarginPct = d.CropMarginPct
	}
	if p.CropMaxPx <= 0 {
		p.CropMaxPx = d.CropMaxPx
	}
	if p.KeepJourneys <= 0 {
		p.KeepJourneys = d.KeepJourneys
	}
	return p
}

// DefaultPictures are starting points, to tune on real runs.
func DefaultPictures() PicturesSettings {
	return PicturesSettings{
		IsolationPx: 12, SettleFrames: 3, MaxDegPerFrame: 6, MinSharpness: 40, SizeFactor: 1.6,
		KeepEveryDeg: 15, MaxKept: 12, CropMarginPct: 20, CropMaxPx: 512, KeepJourneys: 200,
	}
}

// LightsSettings is the machine's light: an [output_pin] in the board config (the
// RAMPS D10 MOSFET), switched on and off.
type LightsSettings struct {
	// Output is the config section, such as "output_pin leds"; empty means no light.
	Output string `json:"output"`
	// OnWhenSorting turns the light on when sorting starts or resumes; the cameras
	// need it.
	OnWhenSorting bool `json:"onWhenSorting"`
}

// CameraSettings say which camera watches each channel (c2, c3, c4) and where that
// channel's zones are in its picture.
type CameraSettings struct {
	// Devices maps a channel to a capture device name (DirectShow on Windows).
	Devices map[string]string `json:"devices"`
	// Zones maps a channel to its ring and zones in that camera's frame.
	Zones  map[string]vision.ChannelZones `json:"zones"`
	Width  int                            `json:"width"`
	Height int                            `json:"height"`
	Fps    int                            `json:"fps"`
	// Ffmpeg is the ffmpeg executable; empty uses the one on PATH.
	Ffmpeg string `json:"ffmpeg"`
	// The detector: a pixel is a piece when |ΔR|+|ΔG|+|ΔB| from the empty channel
	// reaches Threshold, and pieces smaller than MinArea pixels are noise. 0 uses the
	// defaults.
	Threshold int `json:"threshold"`
	MinArea   int `json:"minArea"`
}

// FeederSettings tune the pulsing feeder (C1–C3), after Sorter V2's "Feeder simple
// pulse" page. Distances are rotor degrees; speeds come from feederRpm.
type FeederSettings struct {
	DropPulseDeg float64 `json:"dropPulseDeg"` // nudge for a piece short of the exit
	DropPauseMs  int     `json:"dropPauseMs"`  // look again after a drop-zone nudge
	ExitPulseDeg float64 `json:"exitPulseDeg"` // nudge for a piece at the exit
	ExitPauseMs  int     `json:"exitPauseMs"`
	// HoldAfterFallMs is how long an exit pushes nothing after a piece falls off it,
	// so the next channel sees it land before another can follow.
	HoldAfterFallMs int     `json:"holdAfterFallMs"`
	MinMoveDeg      float64 `json:"minMoveDeg"` // smaller moves are skipped
	MaxMoveDeg      float64 `json:"maxMoveDeg"` // no single pulse is longer
	// C1 has no camera: it advances the bulk pile by this much whenever C2's drop
	// zone is clear, then waits.
	BulkPulseDeg float64 `json:"bulkPulseDeg"`
	BulkPauseMs  int     `json:"bulkPauseMs"`
	// StaleMs: a camera frame older than this is no information.
	StaleMs int `json:"staleMs"`
}

// PlatterSettings tune the classification platter (C4).
type PlatterSettings struct {
	Sectors int `json:"sectors"` // equal sectors; one index turns one sector
	// ExitSector is how many sectors forward of the drop sector the exit is: a
	// piece dropped onto the platter leaves after this many indexes.
	ExitSector int `json:"exitSector"`
	// IdentifyTimeoutMs: a piece still waiting for Brickognize when it reaches the
	// exit goes to misc.
	IdentifyTimeoutMs int `json:"identifyTimeoutMs"`
	// SettleMs is the wait after an index before photographing or trusting the
	// camera.
	SettleMs int `json:"settleMs"`
	// BurstFrames is how many photos of each piece go to Brickognize (≤ 8).
	BurstFrames int `json:"burstFrames"`
	// StallMs: no progress for this long with a piece aboard raises an incident.
	StallMs int `json:"stallMs"`
	// IdleIndexMs: with pieces aboard and none landing for this long, index anyway
	// so they still get delivered.
	IdleIndexMs int `json:"idleIndexMs"`
}

// ChannelSettings maps one rotating channel to its stepper.
type ChannelSettings struct {
	Stepper string  `json:"stepper"`
	Forward bool    `json:"forward"`
	Gear    float64 `json:"gear"` // motor turns per rotor turn
	// Sections is how many identical sections the rotor is divided into, so its
	// look repeats every 360/Sections degrees (C1–C3: 10). 0 means unknown. C4 uses
	// the platter's sectors.
	Sections int `json:"sections"`
}

// RotorPeriod is how far a channel's rotor turns before it looks the same again:
// one section, or a whole turn if that's unknown.
func (s MachineSettings) RotorPeriod(ch Channel) float64 {
	n := s.Channels[string(ch)].Sections
	if ch == ChannelC4 && s.Platter.Sectors > 0 {
		n = s.Platter.Sectors
	}
	if n <= 0 {
		return 360
	}
	return 360 / float64(n)
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
	// SharedBins lets a category share a bin with others once every bin is taken,
	// instead of going to misc.
	SharedBins bool `json:"sharedBins"`

	Layers []LayerSettings `json:"layers"`
}

// LayerSettings is one distribution layer: its servo (the selected layer's servo goes
// to OpenAngle and the others to ClosedAngle), its bins, and what it takes.
type LayerSettings struct {
	Servo       string  `json:"servo"`
	Bins        int     `json:"bins"`
	OpenAngle   float64 `json:"openAngle"`
	ClosedAngle float64 `json:"closedAngle"`
	// Disabled layers, and disabled sections (from 1) of a layer, get no pieces.
	Disabled         bool  `json:"disabled"`
	DisabledSections []int `json:"disabledSections"`
	// MaxPieceMm is the longest piece this layer's funnels take; 0 uses the
	// default for its bin count (see LayerMaxPieceMm).
	MaxPieceMm float64 `json:"maxPieceMm"`
	// MaxPieces is how many pieces a bin of this layer holds before it's full and
	// takes no more until emptied; 0 is no limit.
	MaxPieces int `json:"maxPieces"`
}

// ChannelGear is every channel's (C1–C4) motor turns per rotor turn: 130t rotor gear ÷ 12t motor gear
// (the 24t idler between them changes direction, not ratio).
const ChannelGear = 130.0 / 12

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
			// Every channel: a 12t motor gear drives the rotor's 130t gear through a 24t idler.
			"c1": {Stepper: "stepper_x", Forward: true, Gear: ChannelGear, Sections: 10},
			"c2": {Stepper: "stepper_y", Forward: true, Gear: ChannelGear, Sections: 10},
			"c3": {Stepper: "stepper_z", Forward: true, Gear: ChannelGear, Sections: 10},
			"c4": {Stepper: "stepper_e0", Forward: true, Gear: ChannelGear},
		},
		// Sorter V2's defaults ("Conservative" exit pulses).
		Feeder: FeederSettings{
			DropPulseDeg: 30, DropPauseMs: 100,
			ExitPulseDeg: 2, ExitPauseMs: 100,
			HoldAfterFallMs: 800,
			MinMoveDeg:      0.1, MaxMoveDeg: 120,
			BulkPulseDeg: 10, BulkPauseMs: 1500,
			StaleMs: 1000,
		},
		Platter: PlatterSettings{
			Sectors: 5, ExitSector: 2,
			IdentifyTimeoutMs: 20000, SettleMs: 250, BurstFrames: 6, StallMs: 30000, IdleIndexMs: 1500,
		},
		Cameras: CameraSettings{
			Devices: map[string]string{},
			Zones:   map[string]vision.ChannelZones{},
			// The cameras' largest common size; lower it on the Cameras page if the
			// laptop can't keep up.
			Width: 1280, Height: 720, Fps: 10,
		},
		Lights:   LightsSettings{Output: "output_pin leds", OnWhenSorting: true},
		Pictures: DefaultPictures(),
		// 10 kΩ from 12 V to the pin, 4.7 kΩ to ground: 12 V reads 3.8 V.
		Supply: SupplySettings{
			Sensor: "voltage_sensor supply", TopOhms: 10000, BottomOhms: 4700,
			ReferenceV: 5, Calibration: 1, MinVolts: 10.5,
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
