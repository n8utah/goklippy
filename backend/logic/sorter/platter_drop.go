package sorter

import (
	"log/slog"
	"time"

	"goklippy/backend/logic/vision"
)

// The turn that drops a piece goes on past its sector by as much as the camera says
// brings that piece to the middle of the exit: pieces land anywhere in the drop
// sector, so a fixed sector leaves one that landed at its back short of the exit.
// Once it has fallen (or been found not to), C4 turns back as far, so its sectors are
// where the slots expect them before anything else happens.

// dropKeepBack: the others aboard stay this far short of the exit after an extra turn.
const dropKeepBack = 3.0

// dropExtra is how much further than a sector the next index goes, so the piece about
// to fall reaches the middle of the exit: 0 with none, none seen, it already there,
// or no exit set. It never brings another piece into the exit, and is at most a sector.
func (p *Platter) dropExtra(obs vision.Observation, settings MachineSettings) float64 {
	exitIdx := p.exitIndex(settings) - 1
	s := p.slots[exitIdx]
	z, ok := settings.Cameras.Zones[string(ChannelC4)]
	width := exitWidth(settings, ChannelC4)
	if s == nil || !obs.Valid || !ok || width <= 0 {
		return 0
	}
	sector := 360 / float64(len(p.slots))
	// Where it should be now: where it landed, carried exitIdx sectors on.
	expected := vision.NormalizeAngle(s.angle + float64(exitIdx)*sector)
	target, found := -1, false
	for i, o := range obs.Pieces {
		if d := angleGap(o.Angle, expected); d < sector/2 && (!found || d < angleGap(obs.Pieces[target].Angle, expected)) {
			target, found = i, true
		}
	}
	if !found {
		return 0
	}
	mid := z.Exit.Start + width/2
	extra := vision.ForwardDeg(obs.Pieces[target].Angle+sector, mid)
	if extra > 180 {
		return 0 // past the middle already (the forward way round is most of a turn)
	}
	extra = min(extra, sector)
	for i, o := range obs.Pieces {
		if i == target {
			continue
		}
		// How far it could go on past the sector before reaching the exit; a piece
		// that will be in or past the exit anyway leaves most of a turn.
		if room := vision.ForwardDeg(o.Angle+sector, z.Exit.Start) - dropKeepBack; room < extra {
			extra = max(0, room)
		}
	}
	return extra
}

// angleGap is the angle between two directions, 0–180.
func angleGap(a, b float64) float64 {
	d := vision.ForwardDeg(a, b)
	return min(d, 360-d)
}

// giveBackTurn turns C4 back by the last drop's extra once its fall has been judged.
// It reports whether the platter is busy with that (nothing else should step).
func (p *Platter) giveBackTurn(now time.Time, settings MachineSettings) bool {
	if p.giveBack == 0 {
		return false
	}
	if p.checkFall {
		return true // still watching the piece fall off
	}
	done, err := p.deps.HW.MoveChannel(ChannelC4, -p.giveBack, settings.IndexRpm)
	if err != nil {
		slog.Warn("platter turn back refused", "error", err)
		return true // tried again next step
	}
	slog.Debug("platter turned back after a drop", "deg", p.giveBack)
	p.backDone, p.giveBack = done, 0
	p.progress(now)
	return true
}
