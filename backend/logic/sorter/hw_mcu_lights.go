package sorter

import (
	"fmt"

	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
)

// lightsOutput finds the light's [output_pin] object.
func (h *MCUHardware) lightsOutput(s *mcu.Session) (int, error) {
	name := h.settings().Lights.Output
	if name == "" {
		return 0, fmt.Errorf("%w: no light output is set under System › Settings › Light", ErrNoLight)
	}
	obj := objectBySection(s, name)
	if obj == nil || obj.Kind != cfgtranslate.KindDigitalOut {
		return 0, fmt.Errorf("%w: add [%s] with pin: ar10 under Machine › Board › Wiring", ErrNoLight, name)
	}
	return obj.OID, nil
}

func (h *MCUHardware) SetLights(on bool) error {
	s, err := h.session()
	if err != nil {
		return err
	}
	oid, err := h.lightsOutput(s)
	if err != nil {
		return err
	}
	if err := s.SetOutputPin(oid, on); err != nil {
		return fmt.Errorf("switch the light: %w", err)
	}
	return nil
}

func (h *MCUHardware) LightsStatus() LightsStatus {
	s, err := h.session()
	if err != nil {
		return LightsStatus{Message: err.Error()}
	}
	oid, err := h.lightsOutput(s)
	if err != nil {
		return LightsStatus{Message: err.Error()}
	}
	on, err := s.OutputPinOn(oid)
	if err != nil {
		return LightsStatus{Message: err.Error()}
	}
	return LightsStatus{On: on}
}
