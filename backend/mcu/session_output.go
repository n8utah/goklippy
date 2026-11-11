package mcu

import (
	"fmt"

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
	defer s.mu.Unlock()
	if s.outputLevels == nil {
		s.outputLevels = make(map[int]bool)
	}
	s.outputLevels[oid] = level
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
