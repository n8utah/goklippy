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
	p.turnDone, p.giveBack = done, 0
	p.progress(now)
	return true
}

// straggler is the piece that should have fallen but is still short of the exit or in
// it, lagging behind the platter: the piece nearest the exit, up to a sector short of
// it, that isn't where another slot's piece should be. With no exit set it's whatever
// is in the exit, and its angle isn't known.
func (p *Platter) straggler(obs vision.Observation, settings MachineSettings) (vision.PieceObs, bool) {
	width := exitWidth(settings, ChannelC4)
	if width <= 0 {
		return vision.PieceObs{}, obs.InExit
	}
	sector := 360 / float64(len(p.slots))
	others := p.othersAt(sector)
	var best vision.PieceObs
	found := false
	for _, o := range obs.Pieces {
		if o.ForwardToExit <= -width || o.ForwardToExit >= sector {
			continue // past the exit, or further back on the platter
		}
		mine := true
		for _, e := range others {
			if angleGap(o.Angle, e) < sector/4 {
				mine = false // another slot's piece, riding where it should
				break
			}
		}
		if mine && (!found || o.ForwardToExit < best.ForwardToExit) {
			best, found = o, true
		}
	}
	return best, found
}

// othersAt is where each piece aboard should be now: where it landed, carried on by
// its slot's sectors and the turns past them not yet given back.
func (p *Platter) othersAt(sector float64) []float64 {
	var out []float64
	for i, s := range p.slots {
		if s != nil {
			out = append(out, vision.NormalizeAngle(s.angle+float64(i)*sector+p.giveBack))
		}
	}
	return out
}

// followTurn is how far C4 turns on after a piece lagging at the exit: as far as
// brings it to the middle of the exit, or Platter.FallNudgeDeg once it's there; never
// taking another slot's piece into the exit, nor more than two sectors past the index
// in all. 0 when it can't turn (no exit set, or no room).
func (p *Platter) followTurn(lagging vision.PieceObs, settings MachineSettings) float64 {
	z, ok := settings.Cameras.Zones[string(ChannelC4)]
	width := exitWidth(settings, ChannelC4)
	if !ok || width <= 0 {
		return 0
	}
	sector := 360 / float64(len(p.slots))
	step := settings.Platter.FallNudgeDeg
	deg := vision.ForwardDeg(lagging.Angle, z.Exit.Start+width/2)
	if deg > 180 {
		deg = step // past the middle and still there: a nudge
	}
	deg = min(max(deg, step), 2*sector-p.giveBack)
	for _, e := range p.othersAt(sector) {
		if room := vision.ForwardDeg(e, z.Exit.Start) - dropKeepBack; room < 180 && room < deg {
			deg = room
		}
	}
	return max(0, deg)
}
