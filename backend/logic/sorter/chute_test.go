package sorter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinPositionDefaults(t *testing.T) {
	d := DefaultMachineSettings().Distributor // 6 sections, 51.75° wide, offset 8.25°
	w := 51.75

	cases := []struct {
		addr               BinAddress
		section, inSection int
		angle              float64
	}{
		{BinAddress{Layer: 1, Bin: 1}, 1, 1, 8.25 + w/6},      // 18 bins: 3 per section
		{BinAddress{Layer: 1, Bin: 3}, 1, 3, 8.25 + 2.5*w/3},  // last bin of section 1
		{BinAddress{Layer: 1, Bin: 4}, 2, 1, 8.25 + 60 + w/6}, // next section
		{BinAddress{Layer: 4, Bin: 1}, 1, 1, 8.25 + w/4},      // 12 bins: 2 per section
		{BinAddress{Layer: 4, Bin: 12}, 6, 2, 8.25 + 300 + 1.5*w/2},
	}
	for _, c := range cases {
		pos, err := d.BinPosition(c.addr)
		require.NoError(t, err, "%+v", c.addr)
		assert.Equal(t, c.section, pos.Section, "%+v", c.addr)
		assert.Equal(t, c.inSection, pos.InSection, "%+v", c.addr)
		assert.InDelta(t, c.angle, pos.Angle, 1e-9, "%+v", c.addr)
		assert.True(t, pos.Reachable, "%+v", c.addr)
	}
}

func TestBinPositionDeadWedge(t *testing.T) {
	d := DefaultMachineSettings().Distributor
	d.FirstOffset = 20 // pushes the last section's end past 350° and round past home
	pos, err := d.BinPosition(BinAddress{Layer: 1, Bin: 18})
	require.NoError(t, err)
	// 20 + 300 + 2.5*51.75/3 = 363.125 -> wraps to 3.125°, reached the long way round.
	assert.InDelta(t, 3.125, pos.Angle, 1e-9)
	assert.True(t, pos.Reachable)

	d.FirstOffset = 8.25
	d.MaxAngle = 340
	pos, err = d.BinPosition(BinAddress{Layer: 1, Bin: 18})
	require.NoError(t, err)
	assert.False(t, pos.Reachable, "%.2f° is past the 340° limit", pos.Angle)
}

func TestBinPositionErrors(t *testing.T) {
	d := DefaultMachineSettings().Distributor
	_, err := d.BinPosition(BinAddress{Layer: 5, Bin: 1})
	assert.Error(t, err)
	_, err = d.BinPosition(BinAddress{Layer: 1, Bin: 19})
	assert.Error(t, err)
	_, err = d.BinPosition(BinAddress{Layer: 1, Bin: 0})
	assert.Error(t, err)

	d.Layers[0].Bins = 16 // not a multiple of 6
	_, err = d.BinPosition(BinAddress{Layer: 1, Bin: 1})
	assert.Error(t, err)

	d = DefaultMachineSettings().Distributor
	d.SectionWidth = 70 // wider than the 60° pitch
	_, err = d.BinPosition(BinAddress{Layer: 1, Bin: 1})
	assert.Error(t, err)
}

func TestLayerBins(t *testing.T) {
	d := DefaultMachineSettings().Distributor
	layers := d.LayerBins()
	require.Len(t, layers, 4)
	assert.Len(t, layers[0], 18)
	assert.Len(t, layers[3], 12)
	for _, layer := range layers {
		for i := 1; i < len(layer); i++ {
			assert.Greater(t, layer[i].Angle, layer[i-1].Angle, "bins count up clockwise")
		}
	}
}

func TestCalibrateChuteRoundTrip(t *testing.T) {
	d := DefaultMachineSettings().Distributor
	d.SectionWidth, d.FirstOffset = 49.5, 11.0
	first, err := d.BinPosition(BinAddress{Layer: 1, Bin: 1})
	require.NoError(t, err)
	last, err := d.BinPosition(BinAddress{Layer: 1, Bin: 3})
	require.NoError(t, err)

	cal, err := CalibrateChute(first.Angle, last.Angle, 3, 6)
	require.NoError(t, err)
	assert.InDelta(t, 49.5, cal.SectionWidth, 1e-9)
	assert.InDelta(t, 11.0, cal.FirstOffset, 1e-9)

	// The same calibration aims a 2-bin layer.
	d.SectionWidth, d.FirstOffset = cal.SectionWidth, cal.FirstOffset
	pos, err := d.BinPosition(BinAddress{Layer: 4, Bin: 2})
	require.NoError(t, err)
	assert.InDelta(t, 11.0+1.5*49.5/2, pos.Angle, 1e-9)
}

func TestCalibrateChuteRejectsBadCaptures(t *testing.T) {
	_, err := CalibrateChute(10, 50, 1, 6)
	assert.Error(t, err, "one bin per section has no spread to measure")
	_, err = CalibrateChute(50, 10, 3, 6)
	assert.Error(t, err, "last must be clockwise of first")
	_, err = CalibrateChute(10, 80, 3, 6)
	assert.Error(t, err, "105° section is wider than the 60° pitch")
}

func TestFirstBinAngle(t *testing.T) {
	d := DefaultMachineSettings().Distributor
	assert.InDelta(t, 8.25+51.75/6, d.FirstBinAngle(), 1e-9)
}
