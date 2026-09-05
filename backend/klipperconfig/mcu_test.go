package klipperconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCUBaud(t *testing.T) {
	f, err := Parse("[mcu]\nserial: COM6\nbaud: 250000\n")
	require.NoError(t, err)
	require.Equal(t, 250000, MCUBaud(f))

	f, err = Parse("[mcu]\nserial: COM6\n")
	require.NoError(t, err)
	require.Equal(t, 0, MCUBaud(f))

	require.Equal(t, 0, MCUBaud(nil))
}
