package sorter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
)

const (
	// chuteBackoffDeg is how far the chute turns away from a switch that already reads
	// triggered before homing.
	chuteBackoffDeg = 20
	// chuteHomeSearchTurns is the longest homing move: from anywhere on its travel
	// the switch is under a turn away.
	chuteHomeSearchTurns = 1.1
)

// chuteEndstop finds the home switch's endstop object.
func (h *MCUHardware) chuteEndstop(s *mcu.Session, settings MachineSettings) (oid int, invert bool, err error) {
	name := settings.Distributor.Endstop
	if name == "" {
		return 0, false, fmt.Errorf("no home switch is set for the chute")
	}
	obj := objectBySection(s, name)
	if obj == nil || obj.Kind != cfgtranslate.KindEndstop {
		return 0, false, fmt.Errorf("the chute's home switch %q is not in the board config; add endstop_pin to %s on Board Setup",
			name, settings.Distributor.Stepper)
	}
	return obj.OID, obj.Meta["invert"] == "true", nil
}

// chuteAngle is the chute's angle from home, and false if it isn't homed (in this
// session).
func (h *MCUHardware) chuteAngle(s *mcu.Session, st stepper) (float64, bool, error) {
	h.mu.Lock()
	ref := h.chute
	h.mu.Unlock()
	if !ref.homed || ref.session != s.ID {
		return 0, false, nil
	}
	pos, err := s.StepperPosition(st.oid)
	if err != nil {
		return 0, false, fmt.Errorf("read the chute position: %w", err)
	}
	return float64(h.clockwiseSteps(ref, st, pos)) / st.stepsPerRev * 360, true, nil
}

// clockwiseSteps converts an MCU position into steps clockwise from home.
func (h *MCUHardware) clockwiseSteps(ref chuteReference, st stepper, pos int64) int64 {
	forwardSteps := ref.dirSign * (pos - ref.zero)
	if !st.forward {
		return -forwardSteps
	}
	return forwardSteps
}

// waitFor waits for d or ctx.
func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h *MCUHardware) HomeChute(ctx context.Context) error {
	s, err := h.session()
	if err != nil {
		return err
	}
	settings := h.settings()
	st, err := h.stepperFor(s, settings, ChannelDistributor)
	if err != nil {
		return err
	}
	endstopOID, invert, err := h.chuteEndstop(s, settings)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.chute = chuteReference{}
	h.mu.Unlock()

	triggered, err := s.EndstopTriggered(endstopOID, invert)
	if err != nil {
		return fmt.Errorf("read the home switch: %w", err)
	}
	if triggered {
		// Sitting on the switch: turn clockwise off it first, or homing would take
		// wherever it stands as 0°.
		d, err := s.StepperMove(st.oid, st.forward, mcu.TrapezoidMove{
			Steps: int(chuteBackoffDeg / 360 * st.stepsPerRev),
			MaxHz: settings.Distributor.HomeRpm / 60 * st.stepsPerRev,
		})
		if err != nil {
			return fmt.Errorf("turn off the home switch: %w", err)
		}
		if err := waitFor(ctx, d); err != nil {
			return err
		}
		if triggered, err = s.EndstopTriggered(endstopOID, invert); err != nil {
			return fmt.Errorf("read the home switch: %w", err)
		}
		if triggered {
			return fmt.Errorf("the home switch still reads triggered after turning %d° away from it; check its wiring, or flip its polarity with ! on endstop_pin", chuteBackoffDeg)
		}
	}

	towardHome := !st.forward // counter-clockwise
	res, err := s.HomeStepper(ctx, mcu.HomeMove{
		StepperOID: st.oid,
		EndstopOID: endstopOID,
		Forward:    towardHome,
		SpeedHz:    settings.Distributor.HomeRpm / 60 * st.stepsPerRev,
		MaxSteps:   int(chuteHomeSearchTurns * st.stepsPerRev),
		Invert:     invert,
	})
	if errors.Is(err, mcu.ErrEndstopNotHit) {
		return fmt.Errorf("the chute turned %.1f turns counter-clockwise without the home switch triggering; check the switch, and that the chute turned counter-clockwise (if not, flip Forward)", chuteHomeSearchTurns)
	}
	if err != nil {
		return fmt.Errorf("home the chute: %w", err)
	}

	// The homing move went in a known motor direction, so its change in position
	// says which way the MCU counts.
	dirSign := int64(1)
	if moved := res.After - res.Before; moved != 0 {
		dirSign = int64(math.Copysign(1, float64(moved)))
		if !towardHome {
			dirSign = -dirSign
		}
	}
	h.mu.Lock()
	h.chute = chuteReference{homed: true, session: s.ID, zero: res.After, dirSign: dirSign}
	delete(h.remainders, ChannelDistributor)
	h.mu.Unlock()
	return nil
}

func (h *MCUHardware) ZeroChute() error {
	s, err := h.session()
	if err != nil {
		return err
	}
	st, err := h.stepperFor(s, h.settings(), ChannelDistributor)
	if err != nil {
		return err
	}
	pos, err := s.StepperPosition(st.oid)
	if err != nil {
		return fmt.Errorf("read the chute position: %w", err)
	}
	h.mu.Lock()
	h.chute = chuteReference{homed: true, session: s.ID, zero: pos, dirSign: 1}
	h.mu.Unlock()
	return nil
}

func (h *MCUHardware) AimChute(angle float64) (<-chan error, error) {
	s, err := h.session()
	if err != nil {
		return nil, err
	}
	settings := h.settings()
	d := settings.Distributor
	if angle < 0 || angle > d.MaxAngle {
		return nil, fmt.Errorf("%.1f° is outside the chute's travel (0–%.0f°)", angle, d.MaxAngle)
	}
	if d.Rpm <= 0 {
		return nil, fmt.Errorf("the chute's speed must be positive")
	}
	st, err := h.stepperFor(s, settings, ChannelDistributor)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	ref := h.chute
	h.mu.Unlock()
	if !ref.homed || ref.session != s.ID {
		return nil, ErrChuteNotHomed
	}
	pos, err := s.StepperPosition(st.oid)
	if err != nil {
		return nil, fmt.Errorf("read the chute position: %w", err)
	}

	target := int64(math.Round(angle / 360 * st.stepsPerRev))
	delta := target - h.clockwiseSteps(ref, st, pos)
	if delta == 0 {
		return doneAfter(0), nil
	}
	dur, err := s.StepperMove(st.oid, (delta > 0) == st.forward, mcu.TrapezoidMove{
		Steps:   int(math.Abs(float64(delta))),
		MaxHz:   d.Rpm / 60 * st.stepsPerRev,
		AccelHz: d.Accel / 60 * st.stepsPerRev,
	})
	if err != nil {
		return nil, fmt.Errorf("aim the chute: %w", err)
	}
	return doneAfter(dur), nil
}

func (h *MCUHardware) ChuteStatus() ChuteStatus {
	s, err := h.session()
	if err != nil {
		return ChuteStatus{Message: err.Error()}
	}
	settings := h.settings()
	var status ChuteStatus
	if oid, invert, err := h.chuteEndstop(s, settings); err != nil {
		status.Message = err.Error()
	} else if triggered, err := s.EndstopTriggered(oid, invert); err == nil {
		status.Endstop = "open"
		if triggered {
			status.Endstop = "triggered"
		}
	}
	st, err := h.stepperFor(s, settings, ChannelDistributor)
	if err != nil {
		status.Message = err.Error()
		return status
	}
	if angle, ok, err := h.chuteAngle(s, st); err == nil && ok {
		status.Homed = true
		status.Angle = &angle
	}
	return status
}
