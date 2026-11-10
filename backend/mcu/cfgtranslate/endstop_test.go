package cfgtranslate

import (
	"testing"

	"goklippy/backend/klipperconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInputPin(t *testing.T) {
	cases := []struct {
		raw            string
		pin            string
		pullUp, invert bool
	}{
		{"ar3", "ar3", false, false},
		{"^ar3", "ar3", true, false},
		{"!ar3", "ar3", false, true},
		{"^!ar3", "ar3", true, true},
		{" !^ ar3", "ar3", true, true},
		{"~ar3", "ar3", false, false},
		{"", "", false, false},
	}
	for _, c := range cases {
		pin, pullUp, invert := parseInputPin(c.raw)
		assert.Equal(t, c.pin, pin, c.raw)
		assert.Equal(t, c.pullUp, pullUp, c.raw)
		assert.Equal(t, c.invert, invert, c.raw)
	}
}

func TestEndstopFromStepperSection(t *testing.T) {
	f, err := klipperconfig.Parse(`
[stepper_e1]
step_pin: ar36
dir_pin: ar34
enable_pin: !ar30
microsteps: 16
endstop_pin: ^!ar3
`)
	require.NoError(t, err)
	result, err := Build(f)
	require.NoError(t, err)

	var endstop *ObjectSpec
	for i := range result.Objects {
		if result.Objects[i].Kind == KindEndstop {
			endstop = &result.Objects[i]
		}
	}
	require.NotNil(t, endstop)
	assert.Equal(t, "stepper_e1_endstop", endstop.Section)
	assert.Equal(t, "ar3", endstop.Meta["pin"])
	assert.Equal(t, "true", endstop.Meta["invert"])
	assert.Contains(t, stringsJoin(result.Commands), "pin=ar3 pull_up=1")
}
