package sorter

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
)

const (
	fullStepsPerRev  = 200
	defaultMicrostep = 16
)

// SessionLister lists live MCU sessions. mcu.Manager satisfies it.
type SessionLister interface {
	List() []*mcu.Session
}

// MCUHardware drives the sorter through the one connected Klipper MCU session.
type MCUHardware struct {
	sessions SessionLister
	settings func() MachineSettings

	mu sync.Mutex
	// remainders carries each channel's fraction of a step between moves, so repeated
	// moves (C4 slots) can't drift with fractional gearing.
	remainders map[Channel]float64
	chute      chuteReference
}

// chuteReference ties the chute's 0° to the MCU's step counter for the distributor.
// Angles are read from that counter, so any move (including the bench page's jogs)
// keeps the chute's angle right.
type chuteReference struct {
	homed   bool
	session string // the session it was homed in; a new connection loses it
	zero    int64  // MCU position at 0°
	// dirSign is +1 when a forward (dir=1) step raises the MCU's position count.
	// Homing measures it; ZeroChute assumes +1.
	dirSign int64
}

func NewMCUHardware(sessions SessionLister, settings func() MachineSettings) *MCUHardware {
	return &MCUHardware{sessions: sessions, settings: settings, remainders: make(map[Channel]float64)}
}

func (h *MCUHardware) session() (*mcu.Session, error) {
	sessions := h.sessions.List()
	switch len(sessions) {
	case 0:
		return nil, ErrNoBoard
	case 1:
		return sessions[0], nil
	default:
		return nil, fmt.Errorf("%d boards connected; the sorter needs exactly one", len(sessions))
	}
}

// stepper is one channel's motor as the MCU sees it.
type stepper struct {
	oid         int
	stepsPerRev float64 // motor microsteps per rotor turn
	forward     bool    // motor direction that turns the rotor forward
}

func (h *MCUHardware) stepperFor(s *mcu.Session, settings MachineSettings, ch Channel) (stepper, error) {
	section, gear, forward := "", 0.0, true
	if ch == ChannelDistributor {
		section, gear, forward = settings.Distributor.Stepper, settings.Distributor.Gear, settings.Distributor.Forward
	} else {
		cfg, ok := settings.Channels[string(ch)]
		if !ok {
			return stepper{}, ErrUnknownChannel
		}
		section, gear, forward = cfg.Stepper, cfg.Gear, cfg.Forward
	}
	if gear <= 0 {
		return stepper{}, fmt.Errorf("%s: gear ratio must be positive", ch)
	}
	for _, obj := range s.Objects {
		if obj.Kind != cfgtranslate.KindStepper || obj.Section != section {
			continue
		}
		micro, err := strconv.Atoi(obj.Meta["microsteps"])
		if err != nil || micro <= 0 {
			micro = defaultMicrostep
		}
		return stepper{oid: obj.OID, stepsPerRev: fullStepsPerRev * float64(micro) * gear, forward: forward}, nil
	}
	return stepper{}, fmt.Errorf("%s: stepper %q is not in the board config", ch, section)
}

func (h *MCUHardware) MoveChannel(ch Channel, deg, rpm float64) (<-chan error, error) {
	return h.RampChannel(ch, deg, rpm, channelAccel(h.settings(), ch))
}

func (h *MCUHardware) RampChannel(ch Channel, deg, rpm, accelRpmS float64) (<-chan error, error) {
	if rpm <= 0 {
		return nil, fmt.Errorf("%s: speed must be positive", ch)
	}
	s, err := h.session()
	if err != nil {
		return nil, err
	}
	settings := h.settings()
	st, err := h.stepperFor(s, settings, ch)
	if err != nil {
		return nil, err
	}
	if ch == ChannelDistributor {
		// Once homed, the chute must stay inside its travel or it hits the stop.
		if angle, ok, err := h.chuteAngle(s, st); err != nil {
			return nil, err
		} else if ok {
			target := angle + deg
			if target < 0 || target > settings.Distributor.MaxAngle {
				return nil, fmt.Errorf("that would turn the chute to %.1f°, outside its travel (0–%.0f°)", target, settings.Distributor.MaxAngle)
			}
		}
	}

	h.mu.Lock()
	wanted := deg/360*st.stepsPerRev + h.remainders[ch]
	steps := math.Round(wanted)
	h.mu.Unlock()
	if steps == 0 {
		return doneAfter(0), nil
	}

	accel := accelRpmS / 60 * st.stepsPerRev
	motorForward := (steps > 0) == st.forward
	d, err := s.StepperMove(st.oid, motorForward, mcu.TrapezoidMove{
		Steps:   int(math.Abs(steps)),
		MaxHz:   rpm / 60 * st.stepsPerRev,
		AccelHz: accel,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ch, err)
	}
	h.mu.Lock()
	h.remainders[ch] = wanted - steps
	h.mu.Unlock()
	return doneAfter(d), nil
}

func (h *MCUHardware) Shake(ch Channel, sh Shake) (<-chan error, error) {
	if sh.Rpm <= 0 || sh.Cycles < 1 {
		return nil, fmt.Errorf("%s: a shake needs a speed and at least one cycle", ch)
	}
	s, err := h.session()
	if err != nil {
		return nil, err
	}
	st, err := h.stepperFor(s, h.settings(), ch)
	if err != nil {
		return nil, err
	}
	steps := int(math.Round(sh.AmplitudeDeg / 360 * st.stepsPerRev))
	if steps < 1 {
		return nil, fmt.Errorf("%s: a %.2f° shake is less than a step", ch, sh.AmplitudeDeg)
	}
	stroke := mcu.TrapezoidMove{Steps: steps, MaxHz: sh.Rpm / 60 * st.stepsPerRev, AccelHz: sh.AccelRpmS / 60 * st.stepsPerRev}
	opsPerCycle, err := mcu.ShakeQueueOps(stroke, 2, s.ClockFreq)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ch, err)
	}
	// Whole cycles per part, each part within the MCU's budget, run one after another.
	perPart := max(1, mcu.MaxShakeQueueOps/opsPerCycle)
	first := min(perPart, sh.Cycles)
	d, err := s.StepperShake(st.oid, st.forward, stroke, 2*first)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ch, err)
	}
	done := make(chan error, 1)
	go func() {
		left := sh.Cycles - first
		for {
			time.Sleep(d)
			if left <= 0 {
				done <- nil
				return
			}
			part := min(perPart, left)
			left -= part
			if d, err = s.StepperShake(st.oid, st.forward, stroke, 2*part); err != nil {
				done <- fmt.Errorf("%s: %w", ch, err)
				return
			}
		}
	}()
	return done, nil
}

func (h *MCUHardware) SelectLayer(layer int) error {
	s, err := h.session()
	if err != nil {
		return err
	}
	layers := h.settings().Distributor.Layers
	if layer < -1 || layer >= len(layers) {
		return fmt.Errorf("layer %d does not exist", layer+1)
	}
	var errs []error
	for i, cfg := range layers {
		if cfg.Servo == "" {
			continue
		}
		obj := objectBySection(s, cfg.Servo)
		if obj == nil {
			errs = append(errs, fmt.Errorf("layer %d: servo %q is not in the board config", i+1, cfg.Servo))
			continue
		}
		angle := cfg.ClosedAngle
		if i == layer {
			angle = cfg.OpenAngle
		}
		if err := s.SetServoAngle(obj.OID, angle); err != nil {
			errs = append(errs, fmt.Errorf("layer %d servo: %w", i+1, err))
		}
	}
	return errors.Join(errs...)
}

func (h *MCUHardware) StopAll() error {
	s, err := h.session()
	if err != nil {
		return err
	}
	settings := h.settings()
	var errs []error
	for _, ch := range Channels {
		st, err := h.stepperFor(s, settings, ch)
		if err != nil {
			continue // not mapped, so not moving
		}
		if err := s.StepperStop(st.oid); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ch, err))
		}
	}
	h.mu.Lock()
	clear(h.remainders) // a stop cuts a move short; the remainder no longer applies
	h.mu.Unlock()
	return errors.Join(errs...)
}

func (h *MCUHardware) Health() HardwareHealth {
	s, err := h.session()
	if err != nil {
		return HardwareHealth{Message: err.Error()}
	}
	health := HardwareHealth{Connected: true, Port: s.Port}
	if s.Conn != nil {
		if reason := s.Conn.ShutdownReason(); reason != "" {
			health.Message = "Board shut down: " + reason
		}
	}
	return health
}

func objectBySection(s *mcu.Session, section string) *cfgtranslate.ObjectSpec {
	for i := range s.Objects {
		if s.Objects[i].Section == section {
			return &s.Objects[i]
		}
	}
	return nil
}
