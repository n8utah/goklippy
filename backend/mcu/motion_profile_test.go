package mcu

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testClockFreq = 16e6

// expandQueueSteps replays queue_step chunks the way the MCU does and returns each
// step's clock.
func expandQueueSteps(steps []queueStep) []float64 {
	var out []float64
	at := 0.0
	for _, q := range steps {
		interval := float64(q.Interval)
		for j := 0; j < q.Count; j++ {
			at += interval
			out = append(out, at)
			interval += float64(q.Add)
		}
	}
	return out
}

func TestStepTimesConstantSpeed(t *testing.T) {
	times := TrapezoidMove{Steps: 4, MaxHz: 1000}.stepTimes()
	assert.InDeltaSlice(t, []float64{0.001, 0.002, 0.003, 0.004}, times, 1e-12)
}

func TestStepTimesTrapezoid(t *testing.T) {
	// 1000 steps/s² to 100 steps/s takes 5 steps and 0.1 s each way.
	m := TrapezoidMove{Steps: 20, MaxHz: 100, AccelHz: 1000}
	times := m.stepTimes()
	require.Len(t, times, 20)
	assert.InDelta(t, 0.1, times[4], 1e-9, "end of the ramp")
	assert.InDelta(t, 0.1+10/100.0, times[14], 1e-9, "end of cruise")
	assert.InDelta(t, 0.1+0.1+0.1, m.Duration(), 1e-9)
	for i := 1; i < len(times); i++ {
		assert.Greater(t, times[i], times[i-1])
	}
	// Symmetric: the first step's wait from rest matches the last step's gap.
	assert.InDelta(t, times[0], times[19]-times[18], 1e-9)
}

func TestStepTimesTriangle(t *testing.T) {
	// Too short to reach MaxHz: peaks halfway.
	m := TrapezoidMove{Steps: 10, MaxHz: 10000, AccelHz: 1000}
	times := m.stepTimes()
	half := math.Sqrt(2 * 5 / 1000.0)
	assert.InDelta(t, half, times[4], 1e-9)
	assert.InDelta(t, 2*half, m.Duration(), 1e-9)
}

func TestCompressStepsStaysWithinTolerance(t *testing.T) {
	cases := []TrapezoidMove{
		{Steps: 1, MaxHz: 500},
		{Steps: 6912, MaxHz: 5760},                  // C4 slot, no ramp
		{Steps: 6912, MaxHz: 5760, AccelHz: 34560},  // C4 slot at 60 rpm/s
		{Steps: 6912, MaxHz: 5760, AccelHz: 3456},   // gentle ramp, triangle
		{Steps: 200000, MaxHz: 20000, AccelHz: 8e4}, // long move, chunks capped at 65535
		{Steps: 50, MaxHz: 2000, StartHz: 200, AccelHz: 20000},
	}
	for _, m := range cases {
		steps, err := moveQueueSteps(m, testClockFreq)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(steps), maxMoveQueueOps)

		got := expandQueueSteps(steps)
		require.Len(t, got, m.Steps)
		tol := defaultStepToleranceSec * testClockFreq
		for i, exact := range m.stepTimes() {
			if !assert.InDelta(t, exact*testClockFreq, got[i], tol+1, "move %+v step %d", m, i) {
				break
			}
		}
		for _, q := range steps {
			assert.GreaterOrEqual(t, q.Interval, 1)
			assert.LessOrEqual(t, q.Count, maxQueueStepCount)
			assert.GreaterOrEqual(t, q.Add, minAddValue)
			assert.LessOrEqual(t, q.Add, maxAddValue)
		}
		assert.InDelta(t, m.Duration()*testClockFreq, float64(queueStepsTicks(steps)), tol+1)
	}
}

func TestCompressStepsConstantSpeedIsOneCommand(t *testing.T) {
	steps, err := moveQueueSteps(TrapezoidMove{Steps: 1000, MaxHz: 1000}, testClockFreq)
	require.NoError(t, err)
	assert.Equal(t, []queueStep{{Interval: 16000, Count: 1000}}, steps)
}

func TestMoveQueueStepsRejectsBadMoves(t *testing.T) {
	_, err := moveQueueSteps(TrapezoidMove{Steps: 0, MaxHz: 100}, testClockFreq)
	assert.Error(t, err)
	_, err = moveQueueSteps(TrapezoidMove{Steps: 10}, testClockFreq)
	assert.Error(t, err)
}
