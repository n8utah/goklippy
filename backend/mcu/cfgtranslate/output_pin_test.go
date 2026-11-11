package cfgtranslate

import (
	"testing"

	"goklippy/backend/klipperconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildText(t *testing.T, text string) (BuildResult, error) {
	t.Helper()
	f, err := klipperconfig.Parse(text)
	require.NoError(t, err)
	return Build(f)
}

func TestOutputPinDefaultsOff(t *testing.T) {
	result, err := buildText(t, "[output_pin leds]\npin: ar10\n")
	require.NoError(t, err)
	assert.Equal(t, []string{"config_digital_out oid=0 pin=ar10 value=0 default_value=0 max_duration=0"}, result.Commands)
	require.Len(t, result.Objects, 1)
	obj := result.Objects[0]
	assert.Equal(t, "output_pin leds", obj.Section)
	assert.Equal(t, KindDigitalOut, obj.Kind)
	assert.Equal(t, "false", obj.Meta["invert"])
	assert.Equal(t, "0", obj.Meta["value"])
}

func TestOutputPinStartValueAndShutdownValue(t *testing.T) {
	result, err := buildText(t, "[output_pin leds]\npin: ar10\nvalue: 1\nshutdown_value: 0\n")
	require.NoError(t, err)
	assert.Equal(t, []string{"config_digital_out oid=0 pin=ar10 value=1 default_value=0 max_duration=0"}, result.Commands)
	assert.Equal(t, "1", result.Objects[0].Meta["value"])
	assert.Equal(t, "0", result.Objects[0].Meta["shutdown_level"])
}

func TestOutputPinInverted(t *testing.T) {
	// "!" drives the pin low for on: on at start is level 0, off at shutdown level 1.
	result, err := buildText(t, "[output_pin leds]\npin: !ar10\nvalue: 1\n")
	require.NoError(t, err)
	assert.Equal(t, []string{"config_digital_out oid=0 pin=ar10 value=0 default_value=1 max_duration=0"}, result.Commands)
	assert.Equal(t, "true", result.Objects[0].Meta["invert"])
	assert.Equal(t, "ar10", result.Objects[0].Meta["pin"])
}

func TestOutputPinRejectsPWMValues(t *testing.T) {
	_, err := buildText(t, "[output_pin leds]\npin: ar10\nvalue: 0.5\n")
	assert.ErrorContains(t, err, "value must be 0 or 1")
}
