package klipperconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMinimalRAMPS(t *testing.T) {
	f, err := LoadFile("../testdata/sample-ramps-minimal.cfg")
	require.NoError(t, err)

	mcu, ok := f.Get("mcu")
	require.True(t, ok)
	require.Equal(t, "250000", mcu.Options["baud"])

	stepper, ok := f.Get("stepper_x")
	require.True(t, ok)
	require.Equal(t, "ar54", stepper.Options["step_pin"])
}

func TestSerializeRoundTrip(t *testing.T) {
	original := `[stepper_x]
step_pin: ar54
dir_pin: ar55

`
	f, err := Parse(original)
	require.NoError(t, err)
	f.SetOption("stepper_x", "enable_pin", "!ar38")

	out := f.Serialize()
	require.Contains(t, out, "step_pin: ar54")
	require.Contains(t, out, "enable_pin: !ar38")
}

func TestSaveFileCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/printer.cfg"

	f, err := Parse("[mcu]\nserial: COM1\n")
	require.NoError(t, err)
	require.NoError(t, SaveFile(f, path))

	f.SetOption("mcu", "baud", "250000")
	require.NoError(t, SaveFile(f, path))

	entries, err := ListConfigFiles(dir)
	require.NoError(t, err)
	require.Contains(t, entries, "printer.cfg")
}
