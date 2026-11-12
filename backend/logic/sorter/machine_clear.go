package sorter

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// clearChannels are the channels Clear runs: the camera channels, whose pieces would
// otherwise be tracked or sorted. C1 holds the bulk, so it stays still.
var clearChannels = []Channel{ChannelC2, ChannelC3, ChannelC4}

// ClearChannels runs C2, C3 and C4 together for Feeder.ClearSeconds, all at
// Feeder.ClearRpm (faster than sorting) and all ramping at Feeder.ClearAccel, so
// everything on them falls off their ends. With the chute in, every layer is closed first, so what
// leaves C4 drops into the bucket under the tower (the unsorted pieces); with it left
// out, it isn't touched. It's a job, so Stop cancels it. While a sort is paused,
// whatever was aboard C4 is recorded as gone to the bucket.
func (m *Machine) ClearChannels(ctx context.Context) error {
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		settings := m.settings()
		secs := settings.Feeder.ClearSeconds
		if secs <= 0 || settings.Feeder.ClearRpm <= 0 {
			return fmt.Errorf("%w: set how long and how fast Clear runs (Settings › Feeder)", ErrInvalidSettings)
		}
		chuteIn := !settings.Distributor.LeftOut
		return m.startJob("Clear C2–C4", func(ctx context.Context, report reportFunc) error {
			total := secs
			if chuteIn {
				total++
				report(1, total, "Closing every layer, so pieces drop into the bucket")
				if err := m.hw.SelectLayer(-1); err != nil {
					return err
				}
				if err := waitFor(ctx, time.Duration(settings.Distributor.DoorSettleMs)*time.Millisecond); err != nil {
					return err
				}
			}
			return m.runClear(ctx, settings, clearChannels, secs, total, report)
		}, func(err error) {
			if m.sort != nil {
				m.sort.platter.Sweep(time.Now().UTC())
			}
		})
	}})
}

// clearDegrees is how far a channel turns in secs at up to rpm, ramping up and down at
// accel (rpm per second; 0 for no ramp). A trapezoid move of d turns takes
// d/v + v/a seconds, so d = v·(secs − v/a).
func clearDegrees(rpm, accel, secs float64) float64 {
	v := rpm / 60 // turns per second
	cruise := secs
	if accel > 0 {
		cruise -= rpm / accel
	}
	return max(0, v*cruise) * 360
}

// channelList names channels for people: "C3 and C4", "C2, C3 and C4".
func channelList(chs []Channel) string {
	names := make([]string, len(chs))
	for i, ch := range chs {
		names[i] = strings.ToUpper(string(ch))
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// runClear turns the channels together at the clear speed for secs seconds,
// reporting each second as steps total-secs+1 … total.
func (m *Machine) runClear(ctx context.Context, settings MachineSettings, chs []Channel, secs, total int, report reportFunc) error {
	finished := make(chan error, len(chs))
	rpm, accel := settings.Feeder.ClearRpm, settings.Feeder.ClearAccel
	deg := clearDegrees(rpm, accel, float64(secs))
	which := channelList(chs)
	for _, ch := range chs {
		done, err := m.hw.RampChannel(ch, deg, rpm, accel)
		if err != nil {
			if stopErr := m.hw.StopAll(); stopErr != nil {
				slog.Warn("clear: motors did not all stop", "error", stopErr)
			}
			return err
		}
		go func() { finished <- <-done }()
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	elapsed := 0
	report(total-secs, total, fmt.Sprintf("Running %s: %d of %d s", which, elapsed, secs))
	for left := len(chs); left > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-finished:
			if err != nil {
				return err
			}
			left--
		case <-tick.C:
			elapsed = min(elapsed+1, secs)
			report(total-secs+elapsed, total, fmt.Sprintf("Running %s: %d of %d s", which, elapsed, secs))
		}
	}
	return nil
}
