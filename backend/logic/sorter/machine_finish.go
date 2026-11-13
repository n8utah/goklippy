package sorter

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"goklippy/backend/logic/vision"
)

// Finishing a run on its own. When C2 has had no piece for Feeder.AutoStopSec (the
// hopper has run dry), the run finishes instead of turning an empty C1 for ever, and
// every piece still on the machine is sorted, not cleared off:
//
//  1. sort: C1 stops feeding; C2, C3 and C4 carry on until every piece they can see
//     is sorted;
//  2. sweep C2, then C3: each turns round once in carry pulses, as the feeder moves
//     a piece (C2 only while C3 can take one, C3 only while C4 can), so a piece its
//     camera missed (a white one on a rim) rides to the exit and onto the next channel
//     like any other, and is sorted;
//  3. C4's strays: a piece C4 sees but isn't tracking (one that bounced into another
//     sector) is brought back to the drop sector a sector at a time, where C4 takes it
//     on as a new piece and sorts it;
//  4. stop.
//
// Between steps everything must have been empty a moment. Finishing stops anyway once
// nothing has happened for Feeder.FinishMaxSec (a stuck piece). Keep feeding
// (Machine.KeepFeeding) goes back to sorting as usual, for a refilled hopper.

// FinishStatus is the run finishing on its own, or how it last did (Done).
type FinishStatus struct {
	Since  time.Time `json:"since"`
	Reason string    `json:"reason"`
	Step   string    `json:"step"`
	Done   bool      `json:"done,omitempty"`
	At     time.Time `json:"at,omitzero"` // when it stopped
	Sorted int       `json:"sorted"`      // pieces sorted while finishing
	// IdleDroppedMs: the empty-hopper time left out of the time spent.
	IdleDroppedMs int64 `json:"idle_dropped_ms"`
}

type finishPhase int

const (
	finishSort finishPhase = iota
	finishSweepC2
	finishSweepC3
	finishStrays
)

type finishing struct {
	FinishStatus
	phase      finishPhase
	sweepFrom  float64 // the sweeping channel's odometer when its sweep began
	strayTurns int     // sectors C4 has turned back for strays
	quietSince time.Time
	// progressAt: when something last happened (a piece sorted, a step begun, what's
	// left changed); FinishMaxSec counts from it.
	progressAt time.Time
	lastLeft   string
	lastRecent int
}

// finishQuiet: everything must look empty this long before the next step.
const finishQuiet = 2 * time.Second

// sweepDeg: how far C2 and C3 each turn while sweeping.
const sweepDeg = 360.0

// checkAutoStop starts finishing once C2 has gone without a piece for AutoStopSec.
func (m *Machine) checkAutoStop(now time.Time, fs FeederSettings) {
	st := m.sort
	if st.finish != nil || st.runID == 0 || fs.AutoStopSec <= 0 {
		return
	}
	last := st.feeder.C2SeenAt()
	if last.Before(st.runStart) {
		last = st.runStart
	}
	idle := now.Sub(last)
	if idle < time.Duration(fs.AutoStopSec)*time.Second {
		return
	}
	st.finish = &finishing{
		FinishStatus: FinishStatus{
			Since:  now.UTC(),
			Reason: fmt.Sprintf("no new pieces on C2 for %s", roundDur(idle)),
			Step:   "sorting what's left on C2–C4; C1 has stopped feeding",
		},
		progressAt: now,
		lastRecent: st.platter.Delivered(),
	}
	// The time since C2's last piece was C1 turning an empty hopper: not sorting, so
	// it's taken out of where the run's time went.
	if st.spent != nil {
		st.finish.IdleDroppedMs = st.spent.rewind().Milliseconds()
	}
	m.changed = true
	slog.Info("finishing the run: C2 has had no pieces", "idle", idle.Round(time.Second))
}

// sweeping is the channel finishing is turning round now, for the feeder ("" none).
func (st *sortStations) sweeping() Channel {
	if st.finish == nil {
		return ""
	}
	switch st.finish.phase {
	case finishSweepC2:
		return ChannelC2
	case finishSweepC3:
		return ChannelC3
	}
	return ""
}

// stepFinish moves the finishing on, once the stations have had their tick.
func (m *Machine) stepFinish(now time.Time, settings MachineSettings) {
	st := m.sort
	f := st.finish
	if f == nil || m.job != nil || m.incident != nil {
		return
	}
	left := m.leftOver(now)
	if delivered := st.platter.Delivered(); delivered != f.lastRecent {
		f.Sorted += delivered - f.lastRecent
		f.lastRecent = delivered
		f.progressAt = now
	}
	if left != f.lastLeft {
		f.lastLeft, f.progressAt = left, now
	}
	if limit := settings.Feeder.FinishMaxSec; limit > 0 && now.Sub(f.progressAt) >= time.Duration(limit)*time.Second {
		result := fmt.Sprintf("stopped: nothing happened for %d s", limit)
		if left != "" {
			result += ", with " + left + " left"
		}
		m.finishRun(now, result)
		return
	}

	// Sweeping: until the channel has turned round, with the feeder sorting whatever
	// it turns up as it goes.
	if ch := st.sweeping(); ch != "" {
		turned := st.feeder.Odometer(ch) - f.sweepFrom
		f.Step = fmt.Sprintf("turning %s round for pieces its camera missed (%.0f° of %.0f°)", upper(ch), min(turned, sweepDeg), sweepDeg)
		if left != "" {
			f.Step += "; sorting " + left
		}
		if turned < sweepDeg {
			f.quietSince = time.Time{}
			return
		}
	}
	if left != "" {
		f.quietSince = time.Time{}
		if st.sweeping() == "" {
			f.Step = "sorting what's left: " + left
		}
		return
	}
	if f.quietSince.IsZero() {
		f.quietSince = now
	}
	if now.Sub(f.quietSince) < finishQuiet {
		return
	}
	f.quietSince = time.Time{}
	f.progressAt = now
	switch f.phase {
	case finishSort:
		m.beginSweep(ChannelC2, finishSweepC2)
	case finishSweepC2:
		m.beginSweep(ChannelC3, finishSweepC3)
	case finishSweepC3:
		f.phase = finishStrays
		fallthrough
	case finishStrays:
		// A stray takes a turn of at most a few sectors to reach the drop sector; allow
		// a few tries.
		sectors := max(2, settings.Platter.Sectors)
		if obs := st.deps.Vision.Observation(ChannelC4); len(obs.Pieces) > 0 && f.strayTurns < 2*sectors {
			back := strayBackSectors(settings, sectors, obs)
			f.strayTurns += back
			m.strayBack(settings, sectors, back)
			return
		}
		result := "everything left was sorted"
		if len(st.deps.Vision.Observation(ChannelC4).Pieces) > 0 {
			result = "C4 still sees a piece it couldn't bring round to sort"
		}
		m.finishRun(now, result)
	}
}

func (m *Machine) beginSweep(ch Channel, phase finishPhase) {
	f := m.sort.finish
	f.phase, f.sweepFrom = phase, m.sort.feeder.Odometer(ch)
	f.Step = fmt.Sprintf("turning %s round for pieces its camera missed", upper(ch))
	slog.Info("finishing: sweeping a channel", "channel", ch)
}

// leftOver says what's still to sort ("" when nothing is): pieces seen on C2 or C3,
// aboard C4, or C4 still busy.
func (m *Machine) leftOver(now time.Time) string {
	st := m.sort
	var parts []string
	for _, ch := range []Channel{ChannelC2, ChannelC3} {
		obs := st.deps.Vision.Observation(ch)
		if !obs.Valid {
			parts = append(parts, "no picture of "+upper(ch))
		} else if n := len(obs.Pieces); n > 0 {
			parts = append(parts, fmt.Sprintf("%d on %s", n, upper(ch)))
		}
	}
	if n := len(st.platter.Aboard()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d aboard C4", n))
	} else if st.platter.Busy(time.Now()) {
		parts = append(parts, "C4 still turning")
	}
	return joinAnd(parts)
}

// strayBackSectors is how many whole sectors C4 turns back to bring the stray nearest
// the drop zone into it. C4's camera only sees pieces between the drop zone and the
// exit, so turning back never carries one past the exit. Whole sectors keep the walls
// where the platter expects them. Without C4's zones, one at a time.
func strayBackSectors(settings MachineSettings, sectors int, obs vision.Observation) int {
	zones, ok := settings.Cameras.Zones[string(ChannelC4)]
	if !ok || !zones.Valid() {
		return 1
	}
	sector := 360 / float64(sectors)
	drop := zones.Drop.Start + vision.ForwardDeg(zones.Drop.Start, zones.Drop.End)/2
	best := 0
	for _, p := range obs.Pieces {
		k := int(math.Round(vision.ForwardDeg(drop, p.Angle) / sector))
		if k >= 1 && k < sectors && (best == 0 || k < best) {
			best = k
		}
	}
	return max(1, best)
}

// strayBack turns C4 back n sectors, so a piece it sees but isn't tracking comes to
// the drop sector, where C4 takes it on as a newly landed piece and sorts it. Only
// with nothing aboard: the sectors' pieces aren't followed backwards.
func (m *Machine) strayBack(settings MachineSettings, sectors, n int) {
	st := m.sort
	st.finish.Step = "turning C4 back to bring a piece it isn't tracking to where pieces land, to sort it"
	err := m.startJob("Finishing: bring a piece on C4 round to be sorted", func(ctx context.Context, report reportFunc) error {
		report(1, 1, fmt.Sprintf("Turning C4 back %d sector(s)", n))
		done, err := m.hw.MoveChannel(ChannelC4, -360*float64(n)/float64(sectors), settings.IndexRpm)
		if err != nil {
			return err
		}
		if err := awaitDone(ctx, done); err != nil {
			return err
		}
		return waitFor(ctx, time.Duration(settings.Platter.SettleMs)*time.Millisecond+recoverySettle)
	}, func(err error) {
		if err != nil {
			slog.Warn("finishing: turning C4 back failed", "error", err)
		}
		if st.finish != nil {
			st.finish.quietSince = time.Time{}
		}
		st.platter.progress(time.Now())
	})
	if err != nil {
		slog.Warn("finishing: couldn't turn C4 back", "error", err)
	}
}

// finishRun stops the run on its own, as Stop does, keeping why for the status.
func (m *Machine) finishRun(now time.Time, result string) {
	st := m.sort
	f := st.finish
	f.Done, f.At = true, now.UTC()
	f.Step = result
	st.lastFinish = &f.FinishStatus
	st.finish = nil
	m.endRun(now.UTC())
	m.lightsOff()
	if err := m.hw.StopAll(); err != nil {
		slog.Warn("finishing: motors did not all stop", "error", err)
	}
	m.setState(StateReady, "")
	slog.Info("run finished on its own", "reason", f.Reason, "result", result, "sorted_while_finishing", f.Sorted)
}

// KeepFeeding stops finishing and goes back to sorting as usual (C1 feeding), for a
// refilled hopper. The idle clock starts again.
func (m *Machine) KeepFeeding(ctx context.Context) error {
	return m.send(ctx, request{do: func() error {
		if m.sort == nil || m.sort.finish == nil {
			return fmt.Errorf("%w: the run isn't finishing", ErrInvalidTransition)
		}
		m.sort.finish = nil
		m.sort.feeder.FeedAgain(time.Now())
		m.changed = true
		slog.Info("finishing called off: feeding again")
		return nil
	}})
}

// finishStatus is what the snapshot shows: finishing now, or how the last run finished
// on its own (until the next starts).
func (st *sortStations) finishStatus() *FinishStatus {
	if st.finish != nil {
		s := st.finish.FinishStatus
		return &s
	}
	if st.lastFinish != nil {
		s := *st.lastFinish
		return &s
	}
	return nil
}

func roundDur(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%.0f min", d.Minutes())
	}
	return fmt.Sprintf("%.0f s", d.Seconds())
}

func upper(ch Channel) string {
	return channelList([]Channel{ch})
}

// joinAnd lists parts for people: "a", "a and b", "a, b and c".
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	out := ""
	for i, p := range parts {
		switch {
		case i == 0:
			out = p
		case i == len(parts)-1:
			out += " and " + p
		default:
			out += ", " + p
		}
	}
	return out
}
