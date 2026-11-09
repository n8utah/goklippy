package mcu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func steadySamples(n int, stepsPerInterval int64) []motionSample {
	base := time.Unix(100, 0)
	out := make([]motionSample, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, motionSample{at: base.Add(time.Duration(i) * motionSamplePeriod), pos: int64(i) * stepsPerInterval})
	}
	return out
}

func TestComputeMotionSteadyForward(t *testing.T) {
	m := computeMotion(3, steadySamples(21, 20))

	assert.Equal(t, 3, m.OID)
	assert.Equal(t, int64(400), m.Position)
	assert.InDelta(t, 400, m.StepsPerSec, 0.01)
	assert.Equal(t, 1, m.Direction)
}

func TestComputeMotionReverse(t *testing.T) {
	m := computeMotion(0, steadySamples(21, -10))

	assert.InDelta(t, -200, m.StepsPerSec, 0.01)
	assert.Equal(t, -1, m.Direction)
}

func TestComputeMotionStopped(t *testing.T) {
	m := computeMotion(0, steadySamples(21, 0))

	assert.Equal(t, 0, m.Direction)
}

func TestNextIndexStepsDoesNotDriftOnFractionalMoves(t *testing.T) {
	s := &Session{}
	total := 0
	for i := 0; i < 5; i++ {
		total += s.nextIndexSteps(1, true, 213.4)
	}
	assert.Equal(t, 1067, total)
}
