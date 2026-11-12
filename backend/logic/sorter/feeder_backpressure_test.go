package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
)

// A piece waiting at C3's exit (the platter can't take it) backs C3 up once it has
// waited BackupMs: C2 stops feeding C3, then C2 backs up and C1 stops feeding C2.
func TestFeederBackPressureCascades(t *testing.T) {
	s := feederSettings()
	s.Feeder.BackupMs = 1000
	t0 := time.Now()
	c3Waiting := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZonePrecise, 5)) }
	c2AtExit := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZoneExit, -1)) }

	// Just started waiting: not backed up yet, so C2 still feeds C3.
	f := NewFeeder()
	moves := movesByChannel(f.Step(FeederInput{Now: t0, C2: c2AtExit(t0), C3: c3Waiting(t0)}, s))
	assert.False(t, f.BackedUp(ChannelC3))
	assert.Equal(t, "exit", moves[ChannelC2].Kind, "a short wait doesn't stop C2")

	// A second later C3 is backed up: C2 holds its piece at the exit.
	f = NewFeeder()
	f.Step(FeederInput{Now: t0, C2: obs(t0, piece(vision.ZoneDrop, 100)), C3: c3Waiting(t0)}, s)
	t1 := t0.Add(1100 * time.Millisecond)
	moves = movesByChannel(f.Step(FeederInput{Now: t1, C2: c2AtExit(t1), C3: c3Waiting(t1)}, s))
	assert.True(t, f.BackedUp(ChannelC3))
	assert.NotContains(t, moves, ChannelC2, "C2 waits for C3")
	assert.False(t, f.BackedUp(ChannelC2), "C2 has only just started waiting")

	// Another second: C2 is backed up too, and C1 stops (its own pause is long over).
	t2 := t1.Add(1100 * time.Millisecond)
	moves = movesByChannel(f.Step(FeederInput{Now: t2, C2: c2AtExit(t2), C3: c3Waiting(t2)}, s))
	assert.True(t, f.BackedUp(ChannelC2))
	assert.Empty(t, moves, "nothing feeds a backed-up line")

	// The platter takes C3's piece: C3 exits, and the line flows again.
	t3 := t2.Add(100 * time.Millisecond)
	moves = movesByChannel(f.Step(FeederInput{Now: t3, C2: c2AtExit(t3), C3: c3Waiting(t3), PlatterReady: true}, s))
	assert.False(t, f.BackedUp(ChannelC3))
	assert.Equal(t, "exit", moves[ChannelC3].Kind)
}

// A piece sitting in C3's drop zone holds C2's exit; once that has lasted BackupMs,
// C1 stops piling more onto C2.
func TestFeederBackPressureFromADropZone(t *testing.T) {
	s := feederSettings()
	s.Feeder.BackupMs = 500
	f := NewFeeder()
	t0 := time.Now()
	c2 := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZonePrecise, 4)) }
	c3 := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZoneDrop, 120)) }

	f.Step(FeederInput{Now: t0, C2: c2(t0), C3: c3(t0)}, s)
	t1 := t0.Add(600 * time.Millisecond)
	moves := movesByChannel(f.Step(FeederInput{Now: t1, C2: c2(t1), C3: c3(t1)}, s))
	assert.True(t, f.BackedUp(ChannelC2))
	assert.NotContains(t, moves, ChannelC1)
	assert.NotContains(t, moves, ChannelC2)
}

// The short hold after a fall is pacing, not a hold-up.
func TestFeederHoldAfterFallIsNotBackPressure(t *testing.T) {
	s := feederSettings()
	s.Feeder.BackupMs = 0
	f := NewFeeder()
	t0 := time.Now()
	f.channels[ChannelC3].holdUntil = t0.Add(time.Second)
	f.Step(FeederInput{Now: t0, C2: obs(t0), C3: obs(t0, piece(vision.ZonePrecise, 3)), PlatterReady: true}, s)
	assert.False(t, f.BackedUp(ChannelC3))
}
