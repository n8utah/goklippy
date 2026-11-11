package mcu

import (
	"testing"

	"goklippy/backend/mcu/cfgtranslate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func outputSession() *Session {
	return &Session{Objects: []cfgtranslate.ObjectSpec{
		{OID: 0, Section: "output_pin leds", Kind: cfgtranslate.KindDigitalOut,
			Meta: map[string]string{"pin": "ar10", "invert": "false", "value": "1", "shutdown_level": "0"}},
		{OID: 1, Section: "output_pin inverted", Kind: cfgtranslate.KindDigitalOut,
			Meta: map[string]string{"pin": "ar9", "invert": "true", "value": "0", "shutdown_level": "1"}},
		{OID: 2, Section: "stepper_x_enable", Kind: cfgtranslate.KindDigitalOut,
			Meta: map[string]string{"pin": "ar38", "stepper": "stepper_x"}},
	}}
}

func TestOutputPinStartsAtConfigValue(t *testing.T) {
	s := outputSession()
	on, err := s.OutputPinOn(0)
	require.NoError(t, err)
	assert.True(t, on, "value: 1")
	on, err = s.OutputPinOn(1)
	require.NoError(t, err)
	assert.False(t, on)
}

func TestOutputPinFollowsLevelsSet(t *testing.T) {
	s := outputSession()
	s.recordOutputLevel(0, false)
	s.recordOutputLevel(1, false) // inverted: low is on
	on, _ := s.OutputPinOn(0)
	assert.False(t, on)
	on, _ = s.OutputPinOn(1)
	assert.True(t, on)

	s.outputsToShutdownValues()
	on, _ = s.OutputPinOn(0)
	assert.False(t, on, "shutdown drives it to its shutdown value")
	on, _ = s.OutputPinOn(1)
	assert.False(t, on, "inverted shutdown level 1 is off")

	s.recordOutputLevel(0, false)
	s.forgetOutputLevels()
	on, _ = s.OutputPinOn(0)
	assert.True(t, on, "a reset reconfigures it to value: 1")
}

func TestOutputPinRefusesStepperEnables(t *testing.T) {
	s := outputSession()
	_, err := s.OutputPinOn(2)
	assert.Error(t, err)
	assert.Error(t, s.SetOutputPin(2, true))
	_, err = s.OutputPinOn(9)
	assert.Error(t, err)
}
