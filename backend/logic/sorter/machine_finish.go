package sorter

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Finishing a run on its own. When C2 has had no piece for Feeder.AutoStopSec (the
// hopper has run dry), the run finishes instead of turning an empty C1 for ever:
//
//  1. C1 stops feeding; C2, C3 and C4 carry on until every piece they can see is
//     sorted and they've been empty a moment;
//  2. C2 and C3 then turn round once together, bringing along any piece their cameras
//     missed (a white one on a rim); what that turns up is sorted like any other;
//  3. a piece C4 sees but isn't tracking (one that bounced into another sector) is
//     turned off into the bucket;
//  4. the run stops.
//
// It gives up and stops after Feeder.FinishMaxSec whatever is left. Keep feeding
// (Machine.KeepFeeding) goes back to sorting as usual, for a refilled hopper.

// FinishStatus is the run finishing on its own, or how it last did (Done).
type FinishStatus struct {
	Since  time.Time `json:"since"`
	Reason string    `json:"reason"`
	Step   string    `json:"step"`
	Done   bool      `json:"done,omitempty"`
	At     time.Time `json:"at,omitzero"` // when it stopped
}

type finishing struct {
	FinishStatus
	sweeps     int
	clearedC4  bool
	quietSince time.Time
}

// finishQuiet: everything must look empty this long before the next step.
const finishQuiet = 2 * time.Second

// finishSweeps: how many times C2 and C3 turn round for pieces their cameras missed.
const finishSweeps = 1

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
	st.finish = &finishing{FinishStatus: FinishStatus{
		Since:  now.UTC(),
		Reason: fmt.Sprintf("no new pieces on C2 for %s", roundDur(idle)),
		Step:   "sorting what's left on C2–C4; C1 has stopped feeding",
	}}
	m.changed = true
	slog.Info("finishing the run: C2 has had no pieces", "idle", idle.Round(time.Second))
}

// stepFinish moves the finishing on, once the stations have had their tick.
func (m *Machine) stepFinish(now time.Time, settings MachineSettings) {
	st := m.sort
	f := st.finish
	if f == nil || m.job != nil || m.incident != nil {
		return
	}
	if limit := settings.Feeder.FinishMaxSec; limit > 0 && now.Sub(f.Since) >= time.Duration(limit)*time.Second {
		result := fmt.Sprintf("stopped after %d s of finishing", limit)
		if left := m.leftOver(now); left != "" {
			result += ", with " + left
		}
		m.finishRun(now, result)
		return
	}
	left := m.leftOver(now)
	if left != "" {
		f.quietSince = time.Time{}
		f.Step = "sorting what's left: " + left
		return
	}
	if f.quietSince.IsZero() {
		f.quietSince = now
	}
	if now.Sub(f.quietSince) < finishQuiet {
		return
	}
	switch {
	case f.sweeps < finishSweeps:
		f.sweeps++
		m.finishSweep(settings)
	case !f.clearedC4 && len(st.deps.Vision.Observation(ChannelC4).Pieces) > 0:
		f.clearedC4 = true
		m.finishClearC4(settings)
	default:
		m.finishRun(now, "everything left was sorted")
	}
}

// leftOver says what's still to sort ("" when nothing is): pieces seen on C2 or C3,
// aboard C4, or C4 still busy.
func (m *Machine) leftOver(now time.Time) string {
	st := m.sort
	var parts []string
	for _, ch := range []Channel{ChannelC2, ChannelC3} {
		obs := st.deps.Vision.Observation(ch)
		if !obs.Valid {
			parts = append(parts, "no picture of "+string(ch))
		} else if n := len(obs.Pieces); n > 0 {
			parts = append(parts, fmt.Sprintf("%d on %s", n, upper(ch)))
		}
	}
	if n := len(st.platter.Aboard()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d aboard C4", n))
	} else if st.platter.Busy(now) {
		parts = append(parts, "C4 still turning")
	}
	return joinAnd(parts)
}

// finishSweep turns C2 and C3 round once together, at their carry speed, for pieces
// their cameras missed.
func (m *Machine) finishSweep(settings MachineSettings) {
	st := m.sort
	st.finish.Step = "turning C2 and C3 round once for any piece the cameras missed"
	err := m.startJob("Finishing: turn C2 and C3 round once", func(ctx context.Context, report reportFunc) error {
		report(1, 1, "Turning C2 and C3 round for any piece the cameras missed")
		var dones []<-chan error
		for _, ch := range []Channel{ChannelC2, ChannelC3} {
			done, err := m.hw.RampChannel(ch, 360, carryRpm(settings, ch), settings.Feeder.CarryAccel)
			if err != nil {
				return err
			}
			dones = append(dones, done)
		}
		for _, done := range dones {
			if err := awaitDone(ctx, done); err != nil {
				return err
			}
		}
		return waitFor(ctx, time.Duration(settings.Platter.SettleMs)*time.Millisecond)
	}, func(err error) {
		if st.finish != nil {
			st.finish.quietSince = time.Time{}
			st.finish.Step = "sorting anything the turn brought along"
		}
		if err != nil {
			slog.Warn("finishing: turning C2 and C3 failed", "error", err)
		}
	})
	if err != nil {
		slog.Warn("finishing: couldn't turn C2 and C3", "error", err)
	}
}

// finishClearC4 turns C4 round once with every layer closed, so a piece it isn't
// tracking drops into the bucket.
func (m *Machine) finishClearC4(settings MachineSettings) {
	st := m.sort
	st.finish.Step = "turning C4 round to drop a piece it isn't tracking into the bucket"
	err := m.startJob("Finishing: clear C4", func(ctx context.Context, report reportFunc) error {
		if !settings.Distributor.LeftOut {
			report(1, 2, "Closing every layer, so it drops into the bucket")
			if err := m.hw.SelectLayer(-1); err != nil {
				return err
			}
			if err := waitFor(ctx, time.Duration(settings.Distributor.DoorSettleMs)*time.Millisecond); err != nil {
				return err
			}
		}
		report(2, 2, "Turning C4 round once")
		done, err := m.hw.MoveChannel(ChannelC4, 360, settings.IndexRpm)
		if err != nil {
			return err
		}
		if err := awaitDone(ctx, done); err != nil {
			return err
		}
		return waitFor(ctx, time.Duration(settings.Platter.SettleMs)*time.Millisecond)
	}, func(err error) {
		st.platter.Sweep(time.Now().UTC())
		if st.finish != nil {
			st.finish.quietSince = time.Time{}
		}
		if err != nil {
			slog.Warn("finishing: clearing C4 failed", "error", err)
		}
	})
	if err != nil {
		slog.Warn("finishing: couldn't clear C4", "error", err)
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
	slog.Info("run finished on its own", "reason", f.Reason, "result", result)
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
