package cfgtranslate

import (
	"testing"

	"goklippy/backend/klipperconfig"

	"github.com/stretchr/testify/require"
)

func TestBuildHardwareCommands(t *testing.T) {
	f, err := klipperconfig.LoadFile("../../testdata/sample-ramps-minimal.cfg")
	require.NoError(t, err)

	result, err := Build(f)
	require.NoError(t, err)
	require.NotEmpty(t, result.Commands)

	joined := stringsJoin(result.Commands)
	require.Contains(t, joined, "config_stepper")
	require.Contains(t, joined, "step_pin=ar54")
	require.Contains(t, joined, "invert_step=0")
	require.Contains(t, joined, "config_analog_in")
	require.Contains(t, joined, "config_digital_out")

	finalized := FinalizeCommands(result.Commands)
	require.Equal(t, "allocate_oids count=5", finalized[0])
	require.Contains(t, finalized[len(finalized)-1], "finalize_config crc=")
}

func TestCountOIDs(t *testing.T) {
	lines := []string{
		"config_stepper oid=0 step_pin=ar54 dir_pin=ar55 invert_step=0 step_pulse_ticks=32",
		"config_analog_in oid=1 pin=analog13",
	}
	require.Equal(t, 2, countOIDs(lines))
}

func stringsJoin(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
