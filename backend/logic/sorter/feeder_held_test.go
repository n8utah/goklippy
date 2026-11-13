package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepEvery runs the feeder n times a second apart (every move long done), with C2
// and C3 as given, and returns the moves of each step.
func stepEvery(f *Feeder, start *time.Time, n int, c2, c3 func(time.Time) vision.Observation, ready bool) [][]FeederMove {
	var out [][]FeederMove
	for range n {
		*start = start.Add(time.Second)
		out = append(out, f.Step(FeederInput{Now: *start, C2: c2(*start), C3: c3(*start), PlatterReady: ready}, feederSettings()))
	}
	return out
}

func empty(at time.Time) vision.Observation { return obs(at) }

func TestHeldAtTheExitOfC3IsStuckAfterEnoughTurning(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	onRim := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZoneExit, -1)) }
	stepEvery(f, &now, 10, empty, onRim, true)
	assert.Empty(t, f.Stuck(), "a few pulses: not yet")
	stepEvery(f, &now, 100, empty, onRim, true)
	assert.Equal(t, []Channel{ChannelC3}, f.Stuck(), "C3 turned well past 4×45° under it")
}

func TestACreepingPieceIsNotHeld(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	fwd := 10.0
	creeping := func(at time.Time) vision.Observation {
		fwd -= 1
		return obs(at, piece(vision.ZonePrecise, fwd))
	}
	stepEvery(f, &now, 40, empty, creeping, true)
	assert.Empty(t, f.Stuck())
}

func TestHeldAtC2sExitTurnsC3ThenGivesUp(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	held := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZoneExit, -1)) }
	free := 0
	for _, moves := range stepEvery(f, &now, 60, held, empty, true) {
		for _, mv := range moves {
			if mv.Kind == "free" {
				require.Equal(t, ChannelC3, mv.Channel)
				assert.Equal(t, freeTurnDeg, mv.Degrees)
				free++
			}
		}
	}
	assert.Equal(t, freeTurns, free, "C3 turned to pull it off, then no more")
	assert.Equal(t, []Channel{ChannelC2}, f.Stuck())
}

func TestHeldWhereItLandsWaitsForItToSettle(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	landed := func(at time.Time) vision.Observation { return obs(at, piece(vision.ZoneDrop, 100)) }
	stepEvery(f, &now, 3, empty, landed, true)
	assert.Empty(t, f.Stuck(), "turned 45° under it already, but it only just landed")
	stepEvery(f, &now, 3, empty, landed, true)
	assert.Equal(t, []Channel{ChannelC3}, f.Stuck())
}

// A piece stuck where pieces land, behind a lead that rides on, is found too, and the
// recovery is told which piece it is.
func TestHeldWhereItLandsBehindAMovingLead(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	stuck := vision.PieceObs{Zone: vision.ZoneDrop, ForwardToExit: 200, TrackID: 9, Box: vision.Box{X1: 300, Y1: 20, X2: 340, Y2: 60}}
	fwd := 120.0
	c2 := func(at time.Time) vision.Observation {
		fwd -= 10 // the lead rides
		return obs(at, vision.PieceObs{Zone: vision.ZoneNone, ForwardToExit: fwd, TrackID: 1}, stuck)
	}
	stepEvery(f, &now, 3, c2, empty, true)
	assert.Empty(t, f.Stuck(), "not long enough yet")
	stepEvery(f, &now, 3, c2, empty, true)
	require.Equal(t, []Channel{ChannelC2}, f.Stuck())
	box, ok := f.StuckBox(ChannelC2)
	require.True(t, ok)
	assert.Equal(t, stuck.Box, box, "the one where pieces land, not the lead")
}

func TestAPieceAtThePictureEdgeIsNotWatched(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	cutOff := func(at time.Time) vision.Observation {
		o := obs(at, vision.PieceObs{Zone: vision.ZoneDrop, ForwardToExit: 100, Box: vision.Box{X1: 0, Y1: 50, X2: 30, Y2: 80}})
		o.FrameW, o.FrameH = 640, 480
		return o
	}
	stepEvery(f, &now, 20, empty, cutOff, true)
	assert.Empty(t, f.Stuck())
}

func TestExitStep(t *testing.T) {
	// No next piece, or the rule off: push as usual.
	assert.Equal(t, exitStepMove{kind: exitStepPush, deg: 5}, exitStep(-1, 0, false, 5, 2, 2, 0))
	assert.Equal(t, exitStepMove{kind: exitStepPush, deg: 5}, exitStep(-1, 3, true, 5, 2, 0, 0))
	// Room: push, but no further than keeps the next piece back.
	assert.Equal(t, exitStepMove{kind: exitStepPush, deg: 3}, exitStep(-1, 5, true, 5, 2, 2, 0))
	// No room, the next piece already too close: back off.
	assert.Equal(t, exitStepMove{kind: exitStepBack, deg: 2}, exitStep(-5, 1, true, 5, 2, 2, 0))
	// No room, not too close: shake the lead.
	assert.Equal(t, exitStepMove{kind: exitStepShake, deg: 2}, exitStep(-5, 2.2, true, 5, 2, 2, 0))
	// Tries used up, or the lead isn't in the opening yet: push.
	assert.Equal(t, exitStepPush, exitStep(-5, 2.2, true, 5, 2, 2, exitMaxTries).kind)
	assert.Equal(t, exitStepPush, exitStep(1, 2.2, true, 5, 2, 2, 0).kind)
	// Backing off would take the lead out of the opening: push.
	assert.Equal(t, exitStepMove{kind: exitStepPush, deg: 5, why: exitStepTooClose}, exitStep(-1, 1, true, 5, 2, 2, 0))
}

func TestFeederShakesTheLeadAndOwesTheForwardHalf(t *testing.T) {
	f := NewFeeder()
	now := time.Now()
	two := func(at time.Time) vision.Observation {
		return obs(at, piece(vision.ZoneExit, -5), piece(vision.ZonePrecise, 2.2))
	}
	steps := stepEvery(f, &now, 2, empty, two, true)
	back := movesByChannel(steps[0])[ChannelC3]
	assert.Equal(t, "back", back.Kind)
	assert.InDelta(t, -2, back.Degrees, 1e-9)
	fwd := movesByChannel(steps[1])[ChannelC3]
	assert.Equal(t, "exit", fwd.Kind)
	assert.InDelta(t, 2, fwd.Degrees, 1e-9, "the shake's forward half")
}

func TestC3StandsOffWhileThePlatterIsBusy(t *testing.T) {
	s := feederSettings() // C3's precise zone is 15°, the stand-off 6°
	now := time.Now()
	in := FeederInput{Now: now, C2: obs(now), C3: obs(now, piece(vision.ZoneNone, 19)), PlatterBusy: true}
	assert.NotContains(t, movesByChannel(NewFeeder().Step(in, s)), ChannelC3, "4° short of the precise zone: within the stand-off")
	in.PlatterBusy = false
	assert.Contains(t, movesByChannel(NewFeeder().Step(in, s)), ChannelC3)
	in.C3, in.PlatterBusy = obs(now, piece(vision.ZoneNone, 40)), true
	assert.InDelta(t, 19, movesByChannel(NewFeeder().Step(in, s))[ChannelC3].Degrees, 1e-9, "carried to the stand-off")
}
