package mcu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionKeepsTheNewestAnalogReading(t *testing.T) {
	s := &Session{}
	_, _, ok := s.AnalogReading(3)
	assert.False(t, ok)

	s.noteMessage(map[string]interface{}{"#name": "analog_in_state", "oid": int64(3), "value": int64(512)})
	s.noteMessage(map[string]interface{}{"#name": "stepper_position", "oid": int64(3), "pos": int64(99)})
	v, _, ok := s.AnalogReading(3)
	assert.True(t, ok)
	assert.InDelta(t, 0.5, v, 1e-9)

	s.noteMessage(map[string]interface{}{"#name": "analog_in_state", "oid": int64(3), "value": int64(768)})
	v, _, _ = s.AnalogReading(3)
	assert.InDelta(t, 0.75, v, 1e-9)
}
