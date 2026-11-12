package vision

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoveredAndClump(t *testing.T) {
	o := Observation{Valid: true, RingRadius: 100, Pieces: []PieceObs{
		{Box: Box{X1: 0, Y1: 0, X2: 40, Y2: 40}},
		{Box: Box{X1: 100, Y1: 100, X2: 140, Y2: 130}},
	}}
	assert.InDelta(t, (1600.0+1200)/(math.Pi*100*100), o.Covered(), 1e-9)
	assert.False(t, o.Clump(o.Pieces[0].Box), "a piece")
	assert.True(t, o.Clump(Box{X1: 0, Y1: 0, X2: 80, Y2: 30}), "longer than 3/4 of the radius: a pile")
	assert.Zero(t, Observation{Valid: true}.Covered(), "no ring known")
	assert.False(t, Observation{}.Clump(Box{X2: 1000, Y2: 1000}))
}
