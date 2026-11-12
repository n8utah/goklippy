package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
)

func TestTimeSpentAddsUpTicksAndSkipsGaps(t *testing.T) {
	ts := newTimeSpent()
	t0 := time.Now()
	ts.last = t0
	dt := ts.tick(t0.Add(50 * time.Millisecond))
	ts.add(ChannelC2, TimeMoving, dt)
	dt = ts.tick(t0.Add(100 * time.Millisecond))
	ts.addAll(TimeStuck, dt)
	dt = ts.tick(t0.Add(10 * time.Second)) // paused in between: not counted
	ts.add(ChannelC2, TimeWaiting, dt)

	snap := ts.snapshot()
	assert.Equal(t, int64(50), snap[ChannelC2][TimeMoving])
	assert.Equal(t, int64(50), snap[ChannelC2][TimeStuck])
	assert.Equal(t, int64(50), snap[ChannelC4][TimeStuck])
	assert.Zero(t, snap[ChannelC2][TimeWaiting])
	snap[ChannelC2][TimeMoving] = 999
	assert.Equal(t, int64(50), ts.ms[ChannelC2][TimeMoving], "the snapshot is a copy")
}

func TestFeederSaysWhatEachChannelIsDoing(t *testing.T) {
	s := feederSettings()
	now := time.Now()

	f := NewFeeder()
	f.Step(FeederInput{Now: now, C2: obs(now), C3: obs(now)}, s)
	assert.Equal(t, TimeMoving, f.State(ChannelC1), "C1 feeds an empty C2")
	assert.Equal(t, TimeEmpty, f.State(ChannelC2))
	assert.Equal(t, TimeEmpty, f.State(ChannelC3))

	f = NewFeeder()
	f.Step(FeederInput{Now: now, C2: obs(now, piece(vision.ZoneDrop, 100)), C3: obs(now, piece(vision.ZonePrecise, 4))}, s)
	assert.Equal(t, TimeWaiting, f.State(ChannelC1), "C2's drop zone is busy")
	assert.Equal(t, TimeMoving, f.State(ChannelC2), "carrying its piece on")
	assert.Equal(t, TimeWaiting, f.State(ChannelC3), "the platter can't take it")

	f.Step(FeederInput{Now: now.Add(10 * time.Millisecond), C2: obs(now), C3: obs(now, piece(vision.ZonePrecise, 4))}, s)
	assert.Equal(t, TimeMoving, f.State(ChannelC2), "mid-pulse")
}
