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
//
// Back-pressure: a channel whose leading piece has waited at its exit for
// Feeder.BackupMs (the channel ahead can't take it, it's holding after a fall, or it's
// stuck) is backed up, and the channel before it stops feeding it until it moves
// again. That channel then backs up in turn, so a hold-up anywhere slows everything
// before it instead of piling pieces onto the waiting channel to be dumped at once.

// FeederMove is one pulse the feeder wants.
type FeederMove struct {
	Channel Channel
	Degrees float64
	Rpm     float64
	Accel   float64 // rpm per second; 0 starts at full speed
	Kind    string  // "drop", "exit" or "bulk"
}

// carryRpm is how fast ch carries pieces along: Feeder.CarryRpm, never slower than the
// channel's own speed.
func carryRpm(s MachineSettings, ch Channel) float64 {
	return max(s.FeederRpm[string(ch)], s.Feeder.CarryRpm[string(ch)])
}

// FeederInput is what the feeder reads each tick.
type FeederInput struct {
	Now    time.Time
	C2, C3 vision.Observation
	// PlatterReady is the classification platter's gate: C3 may drop a piece onto it.
	PlatterReady bool
	// PlatterBusy: the platter is turning or has a piece landing, so C3 keeps its lead
	// Feeder.C3BusyStandoffDeg short of the exit (nothing waits at the lip to fall
	// onto it mid-turn).
	PlatterBusy bool
}

// Feeder decides C1–C3's pulses. It keeps its own timers; feed it every tick.
type Feeder struct {
	channels map[Channel]*feederChannel
	// c2SeenAt: when C2 last had a piece on it; long ago, C2 is starving.
	c2SeenAt time.Time
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
	// severalFell: the last exit pulse sent more than one piece off at once.
	severalFell bool
	// state: what it was doing at the last Step, for the time spent (time_spent.go);
	// doing: the same in a few words, with why (Doing).
	state string
	doing string
	// odometer: how far the channel has turned forward; watch: its leading piece
	// against it, to tell a piece the channel can't move (feeder_held.go).
	odometer  float64
	watch     heldWatch
	landWatch heldWatch // the newest piece where pieces land, when not the lead
	stuck     bool
	stuckBox  vision.Box // the piece that made it stuck
	// Its landing area: occupied since when, and the channel's travel then. Still
	// occupied after it has turned well under it, the piece isn't riding: it hangs off
	// the channel above, which may then nudge to drop it (hanging).
	dropSince time.Time
	dropOdo   float64
	dropID    int
	dropGap   float64
	// One piece in the opening (feeder_held.go): back-offs and shakes made for the
	// lead with this track id, and the forward half of a shake still owed.
	exitTriesID int
	exitTries   int
	shakeOwed   float64
	// Back-pressure: since when the leading piece has waited at the exit with
	// nowhere to go (zero when it isn't), and whether that's lasted BackupMs.
	waitingSince time.Time
	backedUp     bool
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
	f.c2SeenAt = time.Time{}
}

// Stuck lists channels whose leading piece won't ride; they stop pulsing until
// Unstick.
func (f *Feeder) Stuck() []Channel {
	var out []Channel
	for _, ch := range []Channel{ChannelC2, ChannelC3} {
		if f.channels[ch].stuck {
			out = append(out, ch)
		}
	}
	return out
}

// Unstick lets a channel pulse again, after a recovery or the operator freed it.
func (f *Feeder) Unstick(ch Channel) {
	if c, ok := f.channels[ch]; ok {
		c.stuck, c.watch, c.landWatch, c.stuckBox = false, heldWatch{}, heldWatch{}, vision.Box{}
	}
}

// Falls is how many pieces this channel has pushed off its exit.
func (f *Feeder) Falls(ch Channel) int {
	if c, ok := f.channels[ch]; ok {
		return c.falls
	}
	return 0
}

// BackedUp reports whether a channel's leading piece has waited at its exit long
// enough that the channel before it has stopped feeding it.
func (f *Feeder) BackedUp(ch Channel) bool {
	if c, ok := f.channels[ch]; ok {
		return c.backedUp
	}
	return false
}

// noteWaiting updates a channel's back-pressure from its newest observation; canGo is
// whether its exit may push now. It returns whether the channel is backed up.
func (f *Feeder) noteWaiting(ch Channel, obs vision.Observation, canGo bool, now time.Time, fs FeederSettings) bool {
	c := f.channels[ch]
	waiting := c.stuck
	if !waiting && obs.Valid && !canGo {
		if lead, ok := obs.Leading(); ok && (lead.Zone == vision.ZonePrecise || lead.Zone == vision.ZoneExit) {
			waiting = true
		}
	}
	if !waiting {
		c.waitingSince, c.backedUp = time.Time{}, false
		return false
	}
	if c.waitingSince.IsZero() {
		c.waitingSince = now
	}
	c.backedUp = now.Sub(c.waitingSince) >= time.Duration(max(0, fs.BackupMs))*time.Millisecond
	return c.backedUp
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
	// move turns ch; negative degrees turn it back (an exit backing off or shaking).
	move := func(ch Channel, kind string, deg float64, pauseMs int, rpm, accel float64) {
		if rpm <= 0 || math.Abs(deg) < fs.MinMoveDeg {
			return
		}
		deg = math.Copysign(math.Min(math.Abs(deg), fs.MaxMoveDeg), deg)
		c := f.channels[ch]
		c.busyUntil = in.Now.Add(moveDuration(deg, rpm, accel) + time.Duration(pauseMs)*time.Millisecond)
		if deg > 0 {
			c.odometer += deg
		}
		moves = append(moves, FeederMove{Channel: ch, Degrees: deg, Rpm: rpm, Accel: accel, Kind: kind})
	}
	// add moves at the channel's own speed (exit pulses, careful); carry at its carry
	// speed (taking pieces along). Both ramp up and down: a sudden start makes pieces
	// slide and bounce on the rotor instead of riding it.
	add := func(ch Channel, kind string, deg float64, pauseMs int) {
		move(ch, kind, deg, pauseMs, settings.FeederRpm[string(ch)], fs.ExitAccel)
	}
	carry := func(ch Channel, kind string, deg float64, pauseMs int) {
		move(ch, kind, deg, pauseMs, carryRpm(settings, ch), fs.CarryAccel)
	}
	if in.C2.Valid && len(in.C2.Pieces) > 0 {
		f.c2SeenAt = in.Now
	}
	c2Hanging := f.noteLanding(ChannelC2, in.C2, in.Now)
	c3Hanging := f.noteLanding(ChannelC3, in.C3, in.Now)

	// Back-pressure, downstream first: C3 waits on the platter, C2 on C3, C1 on C2.
	// The short hold after a fall is pacing, not a hold-up, so it doesn't count.
	c3BackedUp := f.noteWaiting(ChannelC3, in.C3, in.PlatterReady, in.Now, fs)
	// C3 can take pieces: seen lately, its drop zone clear, not backed up. (Mid-move
	// or holding after a fall it's busy, not held up.)
	c3Accepting := in.C3.Valid && in.Now.Sub(in.C3.At) <= stale(fs) && (!in.C3.InDrop || c3Hanging) && !c3BackedUp &&
		!full(fs, ChannelC3, in.C3)
	c2BackedUp := f.noteWaiting(ChannelC2, in.C2, c3Accepting, in.Now, fs)

	// C3 into the platter, then C2 into C3.
	c3Ready := c3Accepting && f.fresh(in.C3, ChannelC3, in.Now, fs) &&
		!in.Now.Before(f.channels[ChannelC3].holdUntil)
	// Why the channel ahead can't take a piece, for the status line.
	c3Why := func() string {
		switch {
		case !in.C3.Valid || in.Now.Sub(in.C3.At) > stale(fs):
			return "no picture of C3"
		case in.C3.InDrop:
			return "a piece is landing on C3"
		case full(fs, ChannelC3, in.C3):
			return "C3 is full"
		case c3BackedUp:
			return "C3 is backed up"
		}
		return "C3 is busy"
	}
	for _, step := range []struct {
		ch         Channel
		obs        vision.Observation
		downstream bool // its exit may push now
		accepting  bool // the channel ahead is taking pieces: carry further
		why        func() string
	}{
		{ChannelC3, in.C3, in.PlatterReady, !c3BackedUp, func() string { return "C4 can't take a piece yet" }},
		{ChannelC2, in.C2, c3Ready, c3Accepting, c3Why},
	} {
		c := f.channels[step.ch]
		set := func(state, doing string) { c.state, c.doing = state, doing }
		switch {
		case c.stuck:
			set(TimeStuck, "stuck: a piece won't ride")
			continue
		case !f.fresh(step.obs, step.ch, in.Now, fs):
			// Mid-move, or waiting for a picture taken after it: moving. No picture
			// at all: nothing to go on.
			if step.obs.Valid && (in.Now.Before(c.busyUntil) || step.obs.At.Before(c.busyUntil)) {
				if c.state != TimeMoving {
					set(TimeMoving, "moving")
				}
			} else {
				set(TimeEmpty, "no picture from its camera")
			}
			continue
		}
		f.noteFall(c, step.obs, in.Now, fs)
		switch f.checkHeld(step.ch, c, step.obs, in.Now) {
		case heldStuck:
			c.stuck = true
			set(TimeStuck, "stuck: its channel turns but the piece doesn't")
			continue
		case heldFreeTurn:
			// C2's lead hangs onto C3: turn C3 a little to pull it off, unless that
			// would push C3's own lead onto a platter that can't take it.
			set(TimeBusy, "a piece hangs on the exit: turning C3 to pull it off")
			c3Lead, c3Has := in.C3.Leading()
			c3AtExit := c3Has && (c3Lead.Zone == vision.ZonePrecise || c3Lead.Zone == vision.ZoneExit)
			if !in.Now.Before(f.channels[ChannelC3].busyUntil) && (!c3AtExit || in.PlatterReady) {
				add(ChannelC3, "free", freeTurnDeg, fs.ExitPauseMs)
				c.watch.turns++
				c.watch.turnedAt = in.Now
			}
			continue
		}
		lead, ok := step.obs.Leading()
		if !ok {
			set(TimeEmpty, "empty")
			continue
		}
		if lead.Zone == vision.ZonePrecise || lead.Zone == vision.ZoneExit {
			switch {
			case in.Now.Before(c.holdUntil):
				set(TimeBusy, "a piece just dropped: letting it land")
			case step.downstream:
				set(TimeMoving, f.exitMove(step.ch, c, step.obs, lead, fs, add))
			default:
				set(TimeWaiting, "a piece is at the exit; waiting: "+step.why())
			}
			continue
		}
		// Short of the exit: stop the leading piece at the precise zone's start. A
		// piece already just short of it creeps in by an exit pulse, so it can't be
		// left needing a move too small to make. Carried at the carry speed, and
		// further while the channel ahead is taking pieces, since it won't be waiting
		// at the exit long. While the platter is busy C3 stops it further back still.
		clearance := lead.ForwardToExit - preciseWidth(settings, step.ch)
		if step.ch == ChannelC3 && in.PlatterBusy && fs.C3BusyStandoffDeg > 0 {
			clearance -= fs.C3BusyStandoffDeg
			if clearance <= 0 {
				set(TimeWaiting, "holding short of the exit while C4 is busy")
				continue
			}
		}
		reach := fs.DropPulseDeg
		if step.accepting {
			reach = max(reach, fs.CarryPulseDeg)
		}
		set(TimeMoving, "carrying a piece to the exit")
		carry(step.ch, "drop", math.Max(math.Min(reach, clearance), fs.ExitPulseDeg), fs.DropPauseMs)
	}

	// C1, blind: feed while C2's drop zone is clear and C2 isn't backed up.
	c1 := f.channels[ChannelC1]
	set1 := func(state, doing string) { c1.state, c1.doing = state, doing }
	switch {
	case in.Now.Before(c1.busyUntil):
		if c1.state != TimeMoving {
			set1(TimeMoving, "feeding C2")
		}
	case !in.C2.Valid || in.Now.Sub(in.C2.At) > stale(fs):
		set1(TimeEmpty, "no picture of C2 to go on")
	case in.C2.InDrop && c2Hanging && !c2BackedUp && !full(fs, ChannelC2, in.C2):
		// C2 turned under it and it stayed: it hangs off C1, so nudge C1 to drop it.
		add(ChannelC1, "bulk", fs.BulkPulseDeg, fs.BulkPauseMs)
		set1(TimeMoving, "a piece hangs off C1 into C2: nudging C1 to drop it")
	case in.C2.InDrop:
		set1(TimeWaiting, "waiting: a piece is landing on C2")
	case c2BackedUp:
		set1(TimeWaiting, "waiting: C2 is backed up")
	case full(fs, ChannelC2, in.C2):
		set1(TimeWaiting, "waiting: C2 is full")
	case fs.StarveMs > 0 && in.Now.Sub(f.c2SeenAt) >= time.Duration(fs.StarveMs)*time.Millisecond:
		// C2 has had nothing for a while: bigger, faster nudges until it does.
		carry(ChannelC1, "bulk", max(fs.BulkPulseDeg, fs.StarveBulkDeg), fs.BulkPauseMs/2)
		set1(TimeMoving, "feeding C2 harder: it has been empty")
	default:
		add(ChannelC1, "bulk", fs.BulkPulseDeg, fs.BulkPauseMs)
		set1(TimeMoving, "feeding C2")
	}
	return moves
}

// hangingAfter: a landing area still occupied this long, while its channel turned
// hangingAfterDeg under it, holds a piece that isn't riding: it hangs off the channel
// above, and only that one moving will drop it.
const (
	hangingAfter    = 3 * time.Second
	hangingAfterDeg = 20.0
)

// noteLanding follows ch's landing area and reports a piece hanging into it from the
// channel above.
func (f *Feeder) noteLanding(ch Channel, obs vision.Observation, now time.Time) bool {
	c := f.channels[ch]
	// The newest piece where pieces land: one in a stream of pieces each riding off
	// replaces the last, so only the same piece, staying put, counts.
	var p vision.PieceObs
	found := false
	for _, q := range obs.Pieces {
		if q.Zone == vision.ZoneDrop && (!found || q.ForwardToExit > p.ForwardToExit) {
			p, found = q, true
		}
	}
	if !obs.Valid || !found {
		c.dropSince = time.Time{}
		return false
	}
	if c.dropSince.IsZero() || p.TrackID != c.dropID || math.Abs(p.ForwardToExit-c.dropGap) >= heldMovedDeg {
		c.dropSince, c.dropOdo, c.dropID, c.dropGap = now, c.odometer, p.TrackID, p.ForwardToExit
		return false
	}
	return now.Sub(c.dropSince) >= hangingAfter && c.odometer-c.dropOdo >= hangingAfterDeg
}

// Doing is what ch is doing now, in a few words ("carrying a piece to the exit",
// "a piece is at the exit; waiting: C3 is full"), for the status on Run › Sort.
func (f *Feeder) Doing(ch Channel) string {
	if c, ok := f.channels[ch]; ok {
		return c.doing
	}
	return ""
}

// full reports ch holding Feeder.MaxPieces pieces, or pieces covering Feeder.MaxCover
// of its ring (pieces lying together count as one blob, so a count alone misses a
// pile): it takes no more for now.
func full(fs FeederSettings, ch Channel, obs vision.Observation) bool {
	if !obs.Valid {
		return false
	}
	limit := fs.MaxPieces[string(ch)]
	return (limit > 0 && len(obs.Pieces) >= limit) || (fs.MaxCover > 0 && obs.Covered() >= fs.MaxCover)
}

// exitPulse is how far an exit pulse turns. Short of the exit zone (forward > 0) it
// covers Feeder.ExitApproach of the way there, so a piece crosses the precise zone in a
// few nudges rather than many; at the exit it's the careful ExitPulseDeg.
func exitPulse(fs FeederSettings, forward float64) float64 {
	if fs.ExitApproach <= 0 || forward <= fs.ExitPulseDeg {
		return fs.ExitPulseDeg
	}
	return max(fs.ExitPulseDeg, forward*min(fs.ExitApproach, 1))
}

// State is what ch was doing at the last Step (TimeMoving, …), for the time spent.
func (f *Feeder) State(ch Channel) string {
	if c, ok := f.channels[ch]; ok {
		return c.state
	}
	return ""
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
	left := atExitCount(obs)
	fell := left < c.atExit || !ok || lead.ForwardToExit > c.leadForward+fallBackDeg
	if fell {
		c.falls++
		c.holdUntil = now.Add(time.Duration(fs.HoldAfterFallMs) * time.Millisecond)
		// Pieces only leave the exit by falling, so two fewer there is two fallen.
		c.severalFell = c.atExit-left >= 2
	}
}

// exitMove is the exit's next move for a lead in the opening: the forward half of a
// shake still owed, or exitStep's push, back-off or shake (feeder_held.go). It says
// which, for the status line.
func (f *Feeder) exitMove(ch Channel, c *feederChannel, obs vision.Observation, lead vision.PieceObs,
	fs FeederSettings, add func(Channel, string, float64, int)) string {
	push := func(deg float64) {
		c.atExit, c.leadForward, c.exitPending = atExitCount(obs), lead.ForwardToExit, true
		add(ch, "exit", deg, fs.ExitPauseMs)
	}
	if c.shakeOwed > 0 {
		deg := c.shakeOwed
		c.shakeOwed = 0
		push(deg)
		return "shaking the piece at the exit loose (forward)"
	}
	if c.exitTriesID != lead.TrackID {
		c.exitTriesID, c.exitTries = lead.TrackID, 0
	}
	next, hasNext := 0.0, len(obs.Pieces) > 1
	if hasNext {
		next = obs.Pieces[1].ForwardToExit
	}
	st := exitStep(lead.ForwardToExit, next, hasNext, exitPulse(fs, lead.ForwardToExit),
		fs.ExitPulseDeg, fs.ExitClearDeg[string(ch)], c.exitTries)
	switch st.kind {
	case exitStepBack:
		c.exitTries++
		add(ch, "back", -st.deg, fs.ExitPauseMs)
		return "backing off: the next piece is too close behind"
	case exitStepShake:
		c.exitTries++
		c.shakeOwed = st.deg
		add(ch, "back", -st.deg, 0)
		return "shaking the piece at the exit loose (back)"
	}
	push(st.deg)
	if lead.ForwardToExit > 0 {
		return "bringing a piece up to the exit"
	}
	return "pushing a piece off the exit"
}

// TakeSeveralFell reports, once, that ch's last exit pulse sent more than one piece
// off together: the channel after it got a double.
func (f *Feeder) TakeSeveralFell(ch Channel) bool {
	c, ok := f.channels[ch]
	if !ok || !c.severalFell {
		return false
	}
	c.severalFell = false
	return true
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
