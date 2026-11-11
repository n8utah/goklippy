package mcu

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

// Output pins ([output_pin] sections) are on/off outputs such as a light on a RAMPS
// MOSFET. The session remembers the level it last set on each, since the MCU can't be
// asked. "On" is the logical value: a pin written "!ar10" is driven low when on.

// SetOutputPin turns an [output_pin] on or off.
func (s *Session) SetOutputPin(oid int, on bool) error {
	obj, err := s.outputPin(oid)
	if err != nil {
		return err
	}
	if err := s.failIfShutdown(); err != nil {
		return err
	}
	return s.SetDigitalOut(oid, on != (obj.Meta["invert"] == "true"))
}

// OutputPinOn is whether an [output_pin] is on: what it was last set to, or its
// config's starting value if nothing has set it since the board was configured.
func (s *Session) OutputPinOn(oid int) (bool, error) {
	obj, err := s.outputPin(oid)
	if err != nil {
		return false, err
	}
	invert := obj.Meta["invert"] == "true"
	s.mu.RLock()
	level, set := s.outputLevels[oid]
	s.mu.RUnlock()
	if !set {
		return obj.Meta["value"] == "1", nil
	}
	return level != invert, nil
}

// outputPin is an on/off output, but not a stepper's enable pin: those belong to the
// motion code.
func (s *Session) outputPin(oid int) (*cfgtranslate.ObjectSpec, error) {
	obj := s.objectByOID(oid)
	if obj == nil || obj.Kind != cfgtranslate.KindDigitalOut || obj.Meta["stepper"] != "" {
		return nil, fmt.Errorf("oid %d is not an output pin", oid)
	}
	return obj, nil
}

func (s *Session) recordOutputLevel(oid int, level bool) {
	s.mu.Lock()
	if s.outputLevels == nil {
		s.outputLevels = make(map[int]bool)
	}
	s.outputLevels[oid] = level
	if s.outputSent == nil {
		s.outputSent = make(map[int]time.Time)
	}
	s.outputSent[oid] = time.Now()
	s.mu.Unlock()
	if obj := s.objectByOID(oid); obj != nil && obj.Meta["max_duration"] != "" {
		s.startOutputKeeper()
	}
}

// Outputs with maximum_mcu_duration must hear from the host or the board shuts down.
// The keeper resends each one's level at a third of its duration while it's away from
// its shutdown value, so a backend that dies (or a lost USB cable) turns it off.
const (
	outputKeeperTick      = 100 * time.Millisecond
	outputRefreshFraction = 3
)

func (s *Session) startOutputKeeper() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outputStop != nil {
		return
	}
	stop := make(chan struct{})
	s.outputStop = stop
	go s.keepOutputs(stop)
}

func (s *Session) stopOutputKeeper() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outputStop != nil {
		close(s.outputStop)
		s.outputStop = nil
	}
}

func (s *Session) keepOutputs(stop chan struct{}) {
	ticker := time.NewTicker(outputKeeperTick)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if s.resetting.Load() {
			continue // the reset reconfigures every output
		}
		for _, cmd := range s.outputRefreshes(time.Now()) {
			s.mu.RLock()
			conn := s.Conn
			s.mu.RUnlock()
			if conn == nil {
				return
			}
			if err := conn.SendCommand(cmd); err != nil {
				slog.Warn("output refresh failed", "port", s.Port, "command", cmd, "error", err)
			}
		}
	}
}

// outputRefreshes lists the update_digital_out commands due now, and marks them sent.
func (s *Session) outputRefreshes(now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cmds []string
	for _, obj := range s.Objects {
		maxDur, err := strconv.ParseFloat(obj.Meta["max_duration"], 64)
		if err != nil || maxDur <= 0 || obj.Kind != cfgtranslate.KindDigitalOut {
			continue
		}
		level, set := s.outputLevels[obj.OID]
		if !set || level == (obj.Meta["shutdown_level"] == "1") {
			continue // at its shutdown value, the board needs nothing
		}
		every := time.Duration(maxDur / outputRefreshFraction * float64(time.Second))
		if now.Sub(s.outputSent[obj.OID]) < every {
			continue
		}
		cmds = append(cmds, fmt.Sprintf("update_digital_out oid=%d value=%d", obj.OID, boolInt(level)))
		s.outputSent[obj.OID] = now
	}
	return cmds
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// outputsToShutdownValues records what a shutdown did: the MCU drove every output to
// its shutdown value, and clearing the shutdown leaves them there.
func (s *Session) outputsToShutdownValues() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outputLevels == nil {
		s.outputLevels = make(map[int]bool)
	}
	for _, obj := range s.Objects {
		if obj.Kind == cfgtranslate.KindDigitalOut && obj.Meta["shutdown_level"] != "" {
			s.outputLevels[obj.OID] = obj.Meta["shutdown_level"] == "1"
		}
	}
}

// forgetOutputLevels is for a board reset: the config sets every output again.
func (s *Session) forgetOutputLevels() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputLevels = nil
}
