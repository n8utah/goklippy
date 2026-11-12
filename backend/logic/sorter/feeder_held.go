package sorter

import (
	"math"
	"time"

	"goklippy/backend/logic/vision"
)

// A piece its own channel can't move, after Sorter V2's pulse_perception/held.py. A
// piece rides its rotor, so when a channel has turned well under its leading piece
// and the piece hasn't moved, something else holds it:
//
//   - where pieces land, or anywhere short of the exit, it hangs off the channel above
//     or sits on the rim beside the rotor: once it has also stayed put heldLandingFor
//     (a piece that just landed often settles a moment before it rides), the channel
//     is stuck and the machine's recovery takes over;
//   - at the exit it hangs onto the channel below: for C2 the feeder turns C3 a little
//     (freeTurnDeg, up to freeTurns times) to pull it off; C3's channel below is the
//     platter, which turns on its own, so C3 waits through as much turning as the free
//     turns would have taken. Then it's stuck.
//
// A piece whose box touches the picture's edge isn't watched: part of it is out of
// sight, so its center doesn't follow it.

const (
	heldMovedDeg   = 3.0  // it moved this much: it rides its rotor
	heldAfterDeg   = 45.0 // its channel turned this much under it while it didn't
	heldLandingFor = 4 * time.Second
	freeTurnDeg    = 8.0
	freeTurns      = 3
	freeSettle     = time.Second
)

type heldWatch struct {
	id       int
	gap      float64 // its ForwardToExit when watching began
	odometer float64 // the channel's travel then
	since    time.Time
	turns    int // free turns of the channel below so far
	turnedAt time.Time
}

// heldVerdict is what checkHeld concluded about the leading piece.
type heldVerdict int

const (
	heldNot      heldVerdict = iota
	heldStuck                // the channel is stuck: recovery
	heldFreeTurn             // turn the channel below to pull it off the exit
)

// checkHeld watches ch's leading piece against the channel's travel.
func (f *Feeder) checkHeld(ch Channel, c *feederChannel, obs vision.Observation, now time.Time) heldVerdict {
	lead, ok := obs.Leading()
	if !ok {
		c.watch = heldWatch{}
		return heldNot
	}
	if atPictureEdge(lead.Box, obs.FrameW, obs.FrameH) {
		return heldNot
	}
	w := &c.watch
	if w.since.IsZero() || w.id != lead.TrackID || math.Abs(lead.ForwardToExit-w.gap) >= heldMovedDeg {
		*w = heldWatch{id: lead.TrackID, gap: lead.ForwardToExit, odometer: c.odometer, since: now}
		return heldNot
	}
	turned := c.odometer - w.odometer
	if turned < heldAfterDeg || (!w.turnedAt.IsZero() && now.Sub(w.turnedAt) < freeSettle) {
		return heldNot
	}
	if lead.Zone == vision.ZonePrecise || lead.Zone == vision.ZoneExit {
		switch {
		case ch == ChannelC2 && w.turns < freeTurns:
			return heldFreeTurn
		case ch == ChannelC3 && turned < heldAfterDeg*(freeTurns+1):
			return heldNot
		}
		return heldStuck
	}
	if now.Sub(w.since) < heldLandingFor {
		return heldNot
	}
	return heldStuck
}

// atPictureEdge: the box touches the edge of a w×h picture (unknown size: no).
func atPictureEdge(b vision.Box, w, h int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	return b.X1 <= 1 || b.Y1 <= 1 || b.X2 >= w-2 || b.Y2 >= h-2
}

// One piece in the opening at a time, after Sorter V2's pulse_perception/exit_step.py.
// A piece can ride a good way into the exit before it slides off, and each exit pulse
// brings the piece behind it closer; once that one's center is at the opening too,
// the next pulse drops both about half the time (against about one in a hundred
// further back). So while the lead is in the opening, the exit never turns so far that
// the next piece comes within Feeder.ExitClearDeg of the opening. With no room it backs
// off (the next piece already too close) or shakes the lead (back a little, then
// forward as far), up to exitMaxTries times for one lead, then pushes as before.

const (
	exitMaxTries      = 4
	exitMinPushDeg    = 0.5
	exitBackExtraDeg  = 1.0
	exitStepPush      = "push"
	exitStepBack      = "back"
	exitStepShake     = "shake"
	exitStepTooClose  = "too close to part"
	exitStepTriesUsed = "tries used up"
)

type exitStepMove struct {
	kind string
	deg  float64
	why  string // a push that goes ahead with the next piece too close
}

// exitStep is the exit's next move. lead and next are how far short of the opening the
// lead's and the next piece's centers are (negative: in it; hasNext false: no next
// piece), push the exit pulse without this rule, shake the shake's throw, clear the
// distance to keep the next piece back (0: off), tries the back-offs and shakes made
// for this lead so far.
func exitStep(lead, next float64, hasNext bool, push, shake, clear float64, tries int) exitStepMove {
	if clear <= 0 || !hasNext {
		return exitStepMove{kind: exitStepPush, deg: push}
	}
	room := next - clear
	switch {
	case room >= exitMinPushDeg:
		return exitStepMove{kind: exitStepPush, deg: min(push, room)}
	case lead >= 0:
		return exitStepMove{kind: exitStepPush, deg: push, why: "beside the lead"}
	case tries >= exitMaxTries:
		return exitStepMove{kind: exitStepPush, deg: push, why: exitStepTriesUsed}
	case room < 0:
		back := exitBackExtraDeg - room
		if lead+back > -exitBackExtraDeg {
			return exitStepMove{kind: exitStepPush, deg: push, why: exitStepTooClose}
		}
		return exitStepMove{kind: exitStepBack, deg: back}
	}
	return exitStepMove{kind: exitStepShake, deg: shake}
}
