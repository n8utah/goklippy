package profiles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRAMPS14Profile(t *testing.T) {
	p, err := Load("ramps14.yaml")
	require.NoError(t, err)
	require.Equal(t, "ramps14", p.ID)
	require.Contains(t, p.Slots, "stepper_x")
	require.Equal(t, 250000, p.Serial.Baud)
	require.Equal(t, "raster", p.Diagram.ImageType)
	require.Equal(t, "images/ramps14.png", p.Diagram.Image)
	require.Equal(t, float64(858), p.Diagram.Width)
	require.NotEmpty(t, p.Diagram.Hotspots)
}
