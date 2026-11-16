package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// feederSettings are the defaults with the exit pinned as these tests were written
// for: halfway approach, no ramp on careful moves, both exits keeping the next piece
// back. TestFeederDefaultsAreSmooth checks the shipped defaults.
func feederSettings() MachineSettings {
	s := DefaultMachineSettings()
	s.Feeder.ExitAccel = 0
	s.Feeder.ExitClearDeg = map[string]float64{"c2": 2, "c3": 2}
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
	assert.Equal(t, 30.0, moves[ChannelC1].Degrees, "C2 has had nothing yet: starving, so a big nudge")
	assert.Equal(t, 5.0, moves[ChannelC1].Rpm)

	// With pieces on C2 lately, the usual small nudge.
	fed := NewFeeder()
	fed.c2SeenAt = now
	moves = movesByChannel(fed.Step(FeederInput{Now: now, C2: obs(now), C3: obs(now), PlatterReady: true}, feederSettings()))
	assert.Equal(t, 10.0, moves[ChannelC1].Degrees)

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
	assert.Equal(t, FeederMove{Channel: ChannelC2, Degrees: 85, Rpm: 12, Accel: 30, Kind: "drop"}, moves[ChannelC2],
		"C3 is taking pieces: carried fast, all the way to the precise zone (100° less its 15°)")

	f = NewFeeder() // C3's drop zone busy: the same one move; the piece waits at the precise zone
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: obs(now, piece(vision.ZoneDrop, 100)), C3: obs(now, piece(vision.ZoneDrop, 120))}, feederSettings()))
	assert.Equal(t, FeederMove{Channel: ChannelC2, Degrees: 85, Rpm: 12, Accel: 30, Kind: "drop"}, moves[ChannelC2])

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
	assert.Equal(t, FeederMove{Channel: ChannelC3, Degrees: 5, Rpm: 11.25, Kind: "exit"}, moves[ChannelC3],
		"5° short, and with no exit zone drawn pieces fall at its start: 5°")

	// C2's exit waits while C3's drop zone has a piece.
	f = NewFeeder()
	c2 := obs(now, piece(vision.ZoneExit, -2))
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: c2, C3: obs(now, piece(vision.ZoneDrop, 90))}, feederSettings()))
	assert.NotContains(t, moves, ChannelC2)
	f = NewFeeder() // the call above set C3 moving, which also blocks C2
	moves = movesByChannel(f.Step(FeederInput{Now: now, C2: c2, C3: obs(now)}, feederSettings()))
	assert.Equal(t, "exit", moves[ChannelC2].Kind)
}

// withExit gives C3 a 20° exit zone, so its pieces start out falling 10° in.
func withExit(s MachineSettings) MachineSettings {
	s.Cameras.Zones = map[string]vision.ChannelZones{
		"c2": {PreciseWidth: 15},
		"c3": {PreciseWidth: 15, Exit: vision.Span{Start: 0, End: 20}},
	}
	return s
}

// c3Step is one feeder step with only C3's picture mattering.
func c3Step(f *Feeder, s MachineSettings, at time.Time, pieces ...vision.PieceObs) FeederMove {
	return movesByChannel(f.Step(FeederInput{Now: at, C2: obs(at), C3: obs(at, pieces...), PlatterReady: true}, s))[ChannelC3]
}

func TestFeederExitGoesToWherePiecesFall(t *testing.T) {
	s := withExit(feederSettings())
	now := time.Now()
	assert.InDelta(t, 22, c3Step(NewFeeder(), s, now, piece(vision.ZonePrecise, 12)).Degrees, 1e-9,
		"12° to the exit and 10° into it, in one move")
	assert.InDelta(t, 18, c3Step(NewFeeder(), s, now, piece(vision.ZonePrecise, 12), piece(vision.ZoneDrop, 20)).Degrees, 1e-9,
		"no further than keeps the next piece 2° short of the exit")
	assert.InDelta(t, 2, c3Step(NewFeeder(), s, now, piece(vision.ZoneExit, -11)).Degrees, 1e-9,
		"past where pieces fall: the careful nudge")

	// It fell on the aimed move: next time a little less.
	f := NewFeeder()
	c3Step(f, s, now, piece(vision.ZonePrecise, 12))
	c3Step(f, s, now.Add(2*time.Second))
	assert.InDelta(t, 9.5, f.fallPoint(ChannelC3, s), 1e-9)

	// It didn't: just past where it got to, then the careful nudge; it fell on that, so
	// that's where they fall.
	f = NewFeeder()
	c3Step(f, s, now, piece(vision.ZonePrecise, 12))
	nudge := c3Step(f, s, now.Add(2*time.Second), piece(vision.ZoneExit, -10))
	assert.InDelta(t, 10.5, f.fallPoint(ChannelC3, s), 1e-9)
	assert.InDelta(t, 2, nudge.Degrees, 1e-9)
	c3Step(f, s, now.Add(4*time.Second))
	assert.InDelta(t, 12, f.fallPoint(ChannelC3, s), 1e-9)
	assert.Equal(t, 1, f.Falls(ChannelC3))

	// Fell sooner than a short move took it: there.
	f = NewFeeder()
	c3Step(f, s, now, piece(vision.ZoneExit, -5), piece(vision.ZoneDrop, 4))
	c3Step(f, s, now.Add(2*time.Second), piece(vision.ZoneDrop, 2))
	assert.InDelta(t, 7, f.fallPoint(ChannelC3, s), 1e-9, "the next piece let it go 2°, to 7° in")
}

func TestFeederShakesAClumpApartThenNudges(t *testing.T) {
	s := withExit(feederSettings())
	s.Feeder.ClumpShakes, s.Feeder.ClumpShakeDeg = 2, 8
	clump := vision.PieceObs{Zone: vision.ZonePrecise, ForwardToExit: 12, Parts: 2}
	jogged := vision.PieceObs{Zone: vision.ZoneDrop, ForwardToExit: 20, Parts: 2}
	f := NewFeeder()
	at := time.Now()
	for i := range 2 {
		back := c3Step(f, s, at, clump)
		assert.Equal(t, FeederMove{Channel: ChannelC3, Degrees: -8, Rpm: 11.25, Kind: "back"}, back, "shake %d: a sudden jog back", i)
		at = at.Add(2 * time.Second)
		fwd := c3Step(f, s, at, jogged)
		assert.Equal(t, FeederMove{Channel: ChannelC3, Degrees: 8, Rpm: 11.25, Kind: "exit"}, fwd, "and forward, from where the jog left it")
		at = at.Add(2 * time.Second)
	}
	assert.InDelta(t, 2, c3Step(f, s, at, clump).Degrees, 1e-9, "still together: nudged off carefully, not to the fall point")
	assert.Contains(t, f.Doing(ChannelC3), "one at a time")

	// Waiting for C4 is a good time to shake.
	f = NewFeeder()
	moves := movesByChannel(f.Step(FeederInput{Now: at, C2: obs(at), C3: obs(at, clump)}, s))
	assert.InDelta(t, -8, moves[ChannelC3].Degrees, 1e-9)

	// One piece isn't shaken.
	assert.InDelta(t, 22, c3Step(NewFeeder(), s, at, piece(vision.ZonePrecise, 12)).Degrees, 1e-9)
}

// Every move ramps (a sudden start makes pieces slide rather than ride), a piece is
// taken to the exit in one move, and both exits keep the next piece back.
func TestFeederDefaultsAreSmooth(t *testing.T) {
	s := DefaultMachineSettings()
	s.Cameras.Zones = feederSettings().Cameras.Zones
	now := time.Now()
	moves := movesByChannel(NewFeeder().Step(FeederInput{Now: now, C2: obs(now), C3: obs(now, piece(vision.ZonePrecise, 10)), PlatterReady: true}, s))
	exit := moves[ChannelC3]
	assert.Equal(t, "exit", exit.Kind)
	assert.InDelta(t, 10, exit.Degrees, 1e-9, "all the way in one move")
	assert.Positive(t, exit.Accel, "ramped")
	assert.Positive(t, moves[ChannelC1].Accel, "C1's nudge too")
	assert.Equal(t, 3.0, s.Feeder.ExitClearDeg["c3"])
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

func TestFeederShakesC1WhenC2StaysEmpty(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	settings := feederSettings()
	step := func(at time.Time, c2 vision.Observation) map[Channel]FeederMove {
		return movesByChannel(f.Step(FeederInput{Now: at, C2: c2, C3: obs(at), PlatterReady: true}, settings))
	}
	assert.Equal(t, "bulk", step(now, obs(now))[ChannelC1].Kind, "a run starts with nudges")

	later := now.Add(7 * time.Second) // C2 still empty
	assert.Equal(t, FeederMove{Channel: ChannelC1, Kind: "shake", Shake: "heavy"}, step(later, obs(later))[ChannelC1],
		"starving a while: the pile is shaken")
	again := later.Add(3 * time.Second)
	assert.Equal(t, "bulk", step(again, obs(again))[ChannelC1].Kind, "then nudges until the next shake is due")

	settings.Feeder.StarveShake = "off"
	f = NewFeeder()
	step(now, obs(now))
	assert.Equal(t, "bulk", step(later, obs(later))[ChannelC1].Kind, "off: nudges only")
}

// Several pieces in a line round the ring are seen as one long blob, its middle far
// behind its front. It's carried until its front, not its middle, reaches the precise
// zone, and from there the exit takes it: a pulse at a time, only when the channel
// ahead can take a piece.
func TestFeederCarriesALongLineByItsFront(t *testing.T) {
	now := time.Now()
	line := vision.PieceObs{Zone: vision.ZoneDrop, ForwardToExit: 185, AheadDeg: 60, Parts: 2}
	moves := movesByChannel(NewFeeder().Step(FeederInput{Now: now, C2: obs(now, line), C3: obs(now)}, feederSettings()))
	assert.Equal(t, "drop", moves[ChannelC2].Kind)
	assert.InDelta(t, 110, moves[ChannelC2].Degrees, 1e-9, "its front 125° short, less the 15° precise zone; not its middle's 170°")

	// Its front there: at the exit, though its middle is far back.
	there := vision.PieceObs{Zone: vision.ZoneNone, ForwardToExit: 75, AheadDeg: 60, Parts: 2}
	moves = movesByChannel(NewFeeder().Step(FeederInput{Now: now, C2: obs(now, there), C3: obs(now, piece(vision.ZoneDrop, 90))}, feederSettings()))
	assert.Equal(t, "back", moves[ChannelC2].Kind, "C3's drop zone is busy: rather than carrying the line in, it's shaken apart while it waits")
	assert.Less(t, moves[ChannelC2].Degrees, 0.0)
	s := feederSettings()
	s.Feeder.ClumpShakes = 0
	moves = movesByChannel(NewFeeder().Step(FeederInput{Now: now, C2: obs(now, there), C3: obs(now)}, s))
	assert.Equal(t, FeederMove{Channel: ChannelC2, Degrees: 2, Rpm: moves[ChannelC2].Rpm, Kind: "exit"}, moves[ChannelC2],
		"several together: an exit pulse, so they come off one at a time")
}

// Two or three pieces touching in a line are one blob, not flagged as several. Long for
// one piece, it's nudged off a careful pulse at a time rather than taken to the fall
// point, which sent the ones behind off with the front one.
func TestFeederNudgesALongBlobOffCarefully(t *testing.T) {
	s := withExit(feederSettings())
	at := time.Now()
	line := vision.PieceObs{Zone: vision.ZonePrecise, ForwardToExit: 12, AheadDeg: 14}
	assert.InDelta(t, 2, c3Step(NewFeeder(), s, at, line).Degrees, 1e-9)
	f := NewFeeder()
	c3Step(f, s, at, line)
	assert.Contains(t, f.Doing(ChannelC3), "in a line")

	short := vision.PieceObs{Zone: vision.ZonePrecise, ForwardToExit: 12, AheadDeg: 6}
	assert.InDelta(t, 22, c3Step(NewFeeder(), s, at, short).Degrees, 1e-9, "one piece: to the fall point")
	s.Feeder.LongAheadDeg = 0
	assert.InDelta(t, 22, c3Step(NewFeeder(), s, at, line).Degrees, 1e-9, "off: as before")
}
