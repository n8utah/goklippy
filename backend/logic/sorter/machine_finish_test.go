package sorter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A pause isn't C2 going without pieces: after resuming, the idle clock carries on from
// where it stood, so a long pause doesn't finish the run straight away.
func TestAutoStopIgnoresPause(t *testing.T) {
	t0 := time.Now()
	st := &sortStations{feeder: NewFeeder(), platter: &Platter{},runID: 1, runStart: t0}
	m := &Machine{sort: st}
	fs := FeederSettings{AutoStopSec: 30}

	st.feeder.c2SeenAt = t0
	for d := time.Duration(0); d <= 10*time.Second; d += 100 * time.Millisecond {
		m.checkAutoStop(t0.Add(d), fs)
	}
	// Paused for ten minutes: no ticks. Then sorting again.
	resumed := t0.Add(10*time.Second + 10*time.Minute)
	m.checkAutoStop(resumed, fs)
	assert.Nil(t, st.finish, "the pause didn't count as C2 idle")
	assert.Equal(t, resumed.Add(-10*time.Second), st.feeder.C2SeenAt(),"C2's clock moved on by the pause")

	// It still finishes once C2 really has gone 30 s without.
	for d := 100 * time.Millisecond; d <= 21*time.Second && st.finish == nil; d += 100 * time.Millisecond {
		m.checkAutoStop(resumed.Add(d), fs)
	}
	if assert.NotNil(t, st.finish) {
		assert.Contains(t, st.finish.Reason, "30 s")
	}
}
