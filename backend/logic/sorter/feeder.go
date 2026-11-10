package sorter

import (
	"math"
	"time"

	"goklippy/backend/logic/vision"
)

// The pulsing feeder, after Sorter V2's subsystems/feeder/pulse_perception. Each tick,
// per channel, downstream first:
//
//   - leading piece in the precise or exit zone, the next channel ready, and the exit
//     not holding after a fall: an exit pulse (small);
//   - leading piece there but the next channel not ready: hold still;
//   - pieces short of the exit: a drop pulse, cut short so the leading piece stops at
//     the precise zone instead of being shoved through the exit;
//   - nothing on the channel: idle.
//
// C1 has no camera: it nudges the bulk pile whenever C2's drop zone is clear. After
// any move a channel waits for a camera frame taken after the move's pause before
// deciding again, so it never acts on a picture of itself moving.

// FeederMove is one pulse the feeder wants.
type FeederMove struct {
	Channel Channel
	Degrees float64
	Rpm     float64
	Kind    string // "drop", "exit" or "bulk"
}

// FeederInput is what the feeder reads each tick.
type FeederInput struct {
	Now    time.Time
	C2, C3 vision.Observation
	// PlatterReady is the classification platter's gate: C3 may drop a piece onto it.
	PlatterReady bool
}

// Feeder decides C1–C3's pulses. It keeps its own timers; feed it every tick.
type Feeder struct {
	channels map[Channel]*feederChannel
}

type feederChannel struct {
	busyUntil time.Time // move plus its pause
	holdUntil time.Time // exit holding after a fall
	// When the last exit pulse went out: how many pieces were at the exit and how
	// far the leading one was from it. Fewer at the exit afterwards, or a leader
	// further back (pieces only move forward), means the old leader fell.
	atExit      int
	leadForward float64
	exitPending bool
	falls       int
}

func NewFeeder() *Feeder {
	return &Feeder{channels: map[Channel]*feederChannel{
		ChannelC1: {}, ChannelC2: {}, ChannelC3: {},
	}}
}

// Reset forgets timers, as after a stop.
func (f *Feeder) Reset() {
	for _, ch := range f.channels {
		*ch = feederChannel{}
	}
}

// Falls is how many pieces this channel has pushed off its exit.
func (f *Feeder) Falls(ch Channel) int {
	if c, ok := f.channels[ch]; ok {
		return c.falls
	}
	return 0
}

// HoldingUntil is when a channel's exit stops holding after a fall.
func (f *Feeder) HoldingUntil(ch Channel) time.Time {
	if c, ok := f.channels[ch]; ok {
		return c.holdUntil
	}
	return time.Time{}
}

// fallBackDeg: a leader this much further from the exit than the one pulsed is a
// different piece (allowing for detection jitter).
const fallBackDeg = 1.0

// Step returns the pulses to start now, recording them as started.
func (f *Feeder) Step(in FeederInput, settings MachineSettings) []FeederMove {
	fs := settings.Feeder
	var moves []FeederMove
	add := func(ch Channel, kind string, deg float64, pauseMs int) {
		rpm := settings.FeederRpm[string(ch)]
		if rpm <= 0 || deg < fs.MinMoveDeg {
			return
		}
		deg = math.Min(deg, fs.MaxMoveDeg)
		c := f.channels[ch]
		c.busyUntil = in.Now.Add(moveDuration(deg, rpm, 0) + time.Duration(pauseMs)*time.Millisecond)
		moves = append(moves, FeederMove{Channel: ch, Degrees: deg, Rpm: rpm, Kind: kind})
	}

	// C3 into the platter, then C2 into C3.
	c3Ready := f.fresh(in.C3, ChannelC3, in.Now, fs) && !in.C3.InDrop &&
		!in.Now.Before(f.channels[ChannelC3].holdUntil)
	for _, step := range []struct {
		ch         Channel
		obs        vision.Observation
		downstream bool
	}{
		{ChannelC3, in.C3, in.PlatterReady},
		{ChannelC2, in.C2, c3Ready},
	} {
		c := f.channels[step.ch]
		if !f.fresh(step.obs, step.ch, in.Now, fs) {
			continue
		}
		f.noteFall(c, step.obs, in.Now, fs)
		lead, ok := step.obs.Leading()
		if !ok {
			continue
		}
		if lead.Zone == vision.ZonePrecise || lead.Zone == vision.ZoneExit {
			if step.downstream && !in.Now.Before(c.holdUntil) {
				c.atExit, c.leadForward, c.exitPending = atExitCount(step.obs), lead.ForwardToExit, true
				add(step.ch, "exit", fs.ExitPulseDeg, fs.ExitPauseMs)
			}
			continue
		}
		// Short of the exit: stop the leading piece at the precise zone's start. A
		// piece already just short of it creeps in by an exit pulse, so it can't be
		// left needing a move too small to make.
		clearance := lead.ForwardToExit - preciseWidth(settings, step.ch)
		add(step.ch, "drop", math.Max(math.Min(fs.DropPulseDeg, clearance), fs.ExitPulseDeg), fs.DropPauseMs)
	}

	// C1, blind: feed while C2's drop zone is clear.
	c1 := f.channels[ChannelC1]
	if !in.Now.Before(c1.busyUntil) && in.C2.Valid && in.Now.Sub(in.C2.At) <= stale(fs) && !in.C2.InDrop {
		add(ChannelC1, "bulk", fs.BulkPulseDeg, fs.BulkPauseMs)
	}
	return moves
}

// fresh reports whether a channel may decide: idle, with a recent frame taken after
// its last move's pause.
func (f *Feeder) fresh(obs vision.Observation, ch Channel, now time.Time, fs FeederSettings) bool {
	c := f.channels[ch]
	return obs.Valid && !now.Before(c.busyUntil) && !obs.At.Before(c.busyUntil) && now.Sub(obs.At) <= stale(fs)
}

// noteFall starts the hold when the last exit pulse sent a piece off the edge.
func (f *Feeder) noteFall(c *feederChannel, obs vision.Observation, now time.Time, fs FeederSettings) {
	if !c.exitPending {
		return
	}
	c.exitPending = false
	lead, ok := obs.Leading()
	fell := atExitCount(obs) < c.atExit || !ok || lead.ForwardToExit > c.leadForward+fallBackDeg
	if fell {
		c.falls++
		c.holdUntil = now.Add(time.Duration(fs.HoldAfterFallMs) * time.Millisecond)
	}
}

func atExitCount(obs vision.Observation) int {
	n := 0
	for _, p := range obs.Pieces {
		if p.Zone == vision.ZonePrecise || p.Zone == vision.ZoneExit {
			n++
		}
	}
	return n
}

// preciseWidth is the channel's precise zone width; the feeder only knows zones
// through observations, so this is the margin it leaves before the exit.
func preciseWidth(settings MachineSettings, ch Channel) float64 {
	if z, ok := settings.Cameras.Zones[string(ch)]; ok && z.PreciseWidth > 0 {
		return z.PreciseWidth
	}
	return 0
}

func stale(fs FeederSettings) time.Duration {
	if fs.StaleMs <= 0 {
		return time.Second
	}
	return time.Duration(fs.StaleMs) * time.Millisecond
}
