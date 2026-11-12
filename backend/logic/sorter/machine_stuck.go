package sorter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"goklippy/backend/logic/vision"
)

// A piece stuck where pieces land on C2 or C3 (it doesn't ride with the channel) is
// dealt with in steps, each tried only if the one before didn't free it:
//
//  1. Shake (Sorter V2's landing-held procedure), only with Feeder.ShakeBeforePush or
//     pushing off: on the machine a stuck piece is most often caught on a fixed part,
//     where shaking only nudges it and pushing frees it. Also skipped when a piece
//     stuck again near where one was freed within stuckAgainWithin.
//  2. Push it past (Feeder.PushPastStuck): turn the channel before a little so a few
//     pieces drop in behind it, then run this channel and those after it together, so
//     they shove it along. With the chute in every layer is closed first; whatever runs
//     off C4 goes to the bucket. It can't help a piece jammed against a fixed divider,
//     so it's checked: if the piece is still where it was, on to 3.
//  3. Work around it (Feeder.WorkAroundStuck, ignored_spots.go), or with that off stop
//     for the operator.

// stuckAgainWithin: a piece sticking again this soon near where one was freed skips
// the shaking.
const stuckAgainWithin = 2 * time.Minute

type freedSpot struct {
	box vision.Box
	at  time.Time
}

// before is the channel that feeds ch.
func before(ch Channel) Channel {
	switch ch {
	case ChannelC3:
		return ChannelC2
	case ChannelC4:
		return ChannelC3
	default:
		return ChannelC1
	}
}

// pushChannels are ch and the camera channels after it.
func pushChannels(ch Channel) []Channel {
	for i, c := range clearChannels {
		if c == ch {
			return append([]Channel(nil), clearChannels[i:]...)
		}
	}
	return []Channel{ch}
}

// stuckAgain reports a piece stuck at box on ch near where one was freed lately.
func (m *Machine) stuckAgain(ch Channel, box vision.Box, now time.Time) bool {
	f, ok := m.freed[ch]
	return ok && now.Sub(f.at) < stuckAgainWithin && centerDist(box, f.box) <= 2*boxSize(f.box)
}

func (m *Machine) noteFreed(ch Channel, box vision.Box, now time.Time) {
	if m.freed == nil {
		m.freed = map[Channel]freedSpot{}
	}
	m.freed[ch] = freedSpot{box: box, at: now}
}

// recoverLanding frees, pushes past or works around a piece stuck where pieces land on
// ch (see the steps above).
func (m *Machine) recoverLanding(ch Channel) {
	st := m.sort
	settings := m.settings()
	obs := st.deps.Vision.Observation(ch)
	lead, seen := obs.Leading()
	box, area := lead.Box, pieceArea(obs, lead.Box)
	push := seen && settings.Feeder.PushPastStuck && settings.Feeder.PushSeconds > 0 && settings.Feeder.ClearRpm > 0
	shake := !push || (settings.Feeder.ShakeBeforePush && !m.stuckAgain(ch, box, time.Now()))
	runner := m.recovery()

	var freed, pushed bool
	name := fmt.Sprintf("Recovery: %s on %s", LandingHeld.Title, ch)
	if !shake {
		name = fmt.Sprintf("Recovery: push a stuck piece past on %s", ch)
	}
	err := m.startJob(name, func(ctx context.Context, report reportFunc) error {
		if shake {
			var err error
			if freed, err = runner.run(ctx, LandingHeld, ch, report); err != nil || freed {
				return err
			}
		}
		if !push {
			return nil
		}
		pushed = true
		if err := m.pushPast(ctx, ch, settings, report); err != nil {
			return err
		}
		freed = !stillThere(st.deps.Vision.Observation(ch), box, area)
		return nil
	}, func(err error) {
		now := time.Now()
		if pushed && st.platter != nil {
			st.platter.Sweep(now.UTC()) // what was on C4 ran off it to the bucket
		}
		if err == nil && freed {
			m.noteFreed(ch, box, now)
			st.feeder.Unstick(ch)
			slog.Info("stuck piece freed", "channel", ch, "pushed", pushed)
			return
		}
		if m.workAroundStuck(ch) {
			return
		}
		m.raiseIncident(Incident{
			Kind:    IncidentLandingStuck,
			Title:   fmt.Sprintf("A piece is stuck where pieces land on %s", ch),
			Message: "It doesn't move with the channel, and shaking and pushing didn't free it. Take it off, then press Done.",
		})
	})
	if err != nil {
		slog.Warn("couldn't start a recovery", "error", err)
		return
	}
	m.recovering = ch
}

// pushPast drops a few pieces in behind the stuck one and runs ch and the channels
// after it together, so they push it along. Runs as part of a job.
func (m *Machine) pushPast(ctx context.Context, ch Channel, settings MachineSettings, report reportFunc) error {
	fs := settings.Feeder
	chs := pushChannels(ch)
	secs := fs.PushSeconds
	total := secs + 2
	if !settings.Distributor.LeftOut {
		report(1, total, "Closing every layer, so whatever is pushed off C4 drops into the bucket")
		if err := m.hw.SelectLayer(-1); err != nil {
			return err
		}
		if err := waitFor(ctx, time.Duration(settings.Distributor.DoorSettleMs)*time.Millisecond); err != nil {
			return err
		}
	}
	if fs.PushBumpDeg > 0 {
		up := before(ch)
		report(2, total, fmt.Sprintf("Turning %s so a few pieces drop in behind the stuck one", up))
		done, err := m.hw.RampChannel(up, fs.PushBumpDeg, fs.ClearRpm, fs.ClearAccel)
		if err != nil {
			return err
		}
		if err := awaitDone(ctx, done); err != nil {
			return err
		}
		if err := waitFor(ctx, pushLandMs*time.Millisecond); err != nil {
			return err
		}
	}
	if err := m.runClear(ctx, settings, chs, secs, total, report); err != nil {
		return err
	}
	// Look again once the channels have stopped and the camera has a still frame.
	return waitFor(ctx, time.Duration(settings.Platter.SettleMs)*time.Millisecond+recoverySettle)
}

// pushLandMs: wait for the bumped pieces to land before pushing.
const pushLandMs = 500
