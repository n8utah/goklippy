package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func feederSettings() MachineSettings {
	s := DefaultMachineSettings()
	s.Cameras.Zones = map[string]vision.ChannelZones{
		"c2": {PreciseWidth: 15},
		"c3": {PreciseWidth: 15},
	}
	return s
}

// obs builds an observation with pieces at the given (zone, forward-to-exit).
func obs(at time.Time, pieces ...vision.PieceObs) vision.Observation {
	o := vision.Observation{At: at, Valid: true, Pieces: pieces}
	for _, p := range pieces {
		switch p.Zone {
		case vision.ZoneDrop:
			o.InDrop = true
		case vision.ZonePrecise:
			o.InPrecise = true
		case vision.ZoneExit:
			o.InExit = true
		}
	}
	return o
}

func piece(zone vision.Zone, forward float64) vision.PieceObs {
	return vision.PieceObs{Zone: zone, ForwardToExit: forward}
}

func movesByChannel(moves []FeederMove) map[Channel]FeederMove {
	out := map[Channel]FeederMove{}
	for _, m := range moves {
		out[m.Channel] = m
	}
	return out
}

func TestFeederEmptyChannelsOnlyFeedBulk(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	moves := movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now), C3: obs(now), PlatterReady: true}, feederSettings()))
	require.Len(t, moves, 1)
	assert.Equal(t, "bulk", moves[ChannelC1].Kind)
	assert.Equal(t, 10.0, moves[ChannelC1].Degrees)
	assert.Equal(t, 5.0, moves[ChannelC1].Rpm)

	// C1 waits out its pause, and waits while C2's drop zone is busy.
	moves = movesByChannel(f.Step(FeederInput{Now: now.Add(time.Second), C2: obs(now.Add(time.Second)), C3: obs(now)}, feederSettings()))
	assert.NotContains(t, moves, ChannelC1)
	later := now.Add(10 * time.Second)
	busy := obs(later, piece(vision.ZoneDrop, 100))
	moves = movesByChannel(f.Step(FeederInput{Now: later, C2: busy, C3: obs(later)}, feederSettings()))
	assert.NotContains(t, moves, ChannelC1)
}

func TestFeederDropPulseStopsAtThePreciseZone(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	moves := movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now, piece(vision.ZoneDrop, 100)), C3: obs(now)}, feederSettings()))
	assert.Equal(t, FeederMove{Channel: ChannelC2, Degrees: 30, Rpm: 7.5, Kind: "drop"}, moves[ChannelC2])

	f = NewFeeder()
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now, piece(vision.ZoneNone, 21)), C3: obs(now)}, feederSettings()))
	assert.InDelta(t, 6, moves[ChannelC2].Degrees, 1e-9, "21° short of the exit, 15° precise zone")

	f = NewFeeder()
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now, piece(vision.ZoneNone, 15.05)), C3: obs(now)}, feederSettings()))
	assert.InDelta(t, 2, moves[ChannelC2].Degrees, 1e-9, "creeps in rather than sticking")
}

func TestFeederExitWaitsForDownstream(t *testing.T) {
	now := time.Now()
	atExit := obs(now, piece(vision.ZonePrecise, 5))

	f := NewFeeder()
	moves := movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now), C3: atExit, PlatterReady: false}, feederSettings()))
	assert.NotContains(t, moves, ChannelC3, "the platter isn't ready")

	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now), C3: atExit, PlatterReady: true}, feederSettings()))
	assert.Equal(t, FeederMove{Channel: ChannelC3, Degrees: 2, Rpm: 11.25, Kind: "exit"}, moves[ChannelC3])

	// C2's exit waits while C3's drop zone has a piece.
	f = NewFeeder()
	c2 := obs(now, piece(vision.ZoneExit, -2))
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: c2, C3: obs(now, piece(vision.ZoneDrop, 90))}, feederSettings()))
	assert.NotContains(t, moves, ChannelC2)
	f = NewFeeder() // the call above set C3 moving, which also blocks C2
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: c2, C3: obs(now)}, feederSettings()))
	assert.Equal(t, "exit", moves[ChannelC2].Kind)
}

func TestFeederHoldsAfterAFall(t *testing.T) {
	f := NewFeeder()
	s := feederSettings()
	t0 := time.Now()
	in := FeederInput{Now: t0, C2: obs(t0), C3: obs(t0, piece(vision.ZoneExit, -1)), PlatterReady: true}
	require.Contains(t, movesByChannel(f.Step(in, s)), ChannelC3)

	// The next frame after the pulse: the piece has gone, so it fell.
	t1 := t0.Add(300 * time.Millisecond)
	in = FeederInput{Now: t1, C2: obs(t1), C3: obs(t1, piece(vision.ZonePrecise, 3)), PlatterReady: true}
	moves := movesByChannel(f.Step(in, s))
	assert.Equal(t, 1, f.Falls(ChannelC3))
	assert.NotContains(t, moves, ChannelC3, "holding after the fall")

	t2 := t1.Add(time.Second)
	in = FeederInput{Now: t2, C2: obs(t2), C3: obs(t2, piece(vision.ZonePrecise, 3)), PlatterReady: true}
	assert.Contains(t, movesByChannel(f.Step(in, s)), ChannelC3, "the hold is over")
}

func TestFeederWaitsForAFrameAfterItsMove(t *testing.T) {
	f := NewFeeder()
	s := feederSettings()
	t0 := time.Now()
	in := FeederInput{Now: t0, C2: obs(t0, piece(vision.ZoneDrop, 100)), C3: obs(t0)}
	require.Contains(t, movesByChannel(f.Step(in, s)), ChannelC2)

	// Well after the move, but the newest frame is still the old one.
	t1 := t0.Add(5 * time.Second)
	in = FeederInput{Now: t1, C2: obs(t0, piece(vision.ZoneDrop, 100)), C3: obs(t1)}
	assert.NotContains(t, movesByChannel(f.Step(in, s)), ChannelC2)

	in.C2 = obs(t1, piece(vision.ZoneDrop, 70))
	assert.Contains(t, movesByChannel(f.Step(in, s)), ChannelC2)
}

func TestFeederIgnoresMissingOrStaleCameras(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	moves := f.Step(FeederInput{Now: now, PlatterReady: true}, feederSettings())
	assert.Empty(t, moves, "no frames yet: nothing moves, C1 included")

	old := now.Add(-5 * time.Second)
	moves = f.Step(FeederInput{Now: now, C2: obs(old, piece(vision.ZoneDrop, 100)), C3: obs(old)}, feederSettings())
	assert.Empty(t, moves)
}
