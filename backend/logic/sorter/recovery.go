package sorter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"goklippy/backend/logic/vision"
)

// Shakes and recovery procedures, after Sorter V2's recovery/: short, sharp
// back-and-forth runs of a channel that break a stuck piece loose, and lists of moves
// tried in order, checking after each whether the piece is free.

// ShakeProfile is one shake. Amplitude is in motor degrees (before gearing), speed
// and acceleration in motor microsteps at 16 microsteps, as Sorter V2 tuned them.
type ShakeProfile struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Blurb        string  `json:"blurb"`
	AmplitudeDeg float64 `json:"amplitude_deg"`
	Cycles       int     `json:"cycles"`
	Speed        float64 `json:"speed"`        // microsteps per second
	Acceleration float64 `json:"acceleration"` // microsteps per second²
}

// ShakeProfiles, gentlest to strongest. "Heavy stuck" freed the most pieces on
// Sorter V2's test bench.
var ShakeProfiles = []ShakeProfile{
	{"brief", "Brief", "A short, sharp burst", 6, 3, 5000, 100000},
	{"heavy", "Heavy stuck", "Medium throw, medium length", 6, 12, 5000, 100000},
	{"long", "Soft and long", "Gentle but long", 5, 30, 3500, 55000},
	{"hard", "Big and hard", "Big throw, strong jerk", 9, 8, 6500, 200000},
	{"max", "Max shake", "The largest throw, strong", 13, 12, 6500, 190000},
}

var ErrUnknownShake = errors.New("no shake by that name")

func shakeProfile(key string) (ShakeProfile, error) {
	for _, p := range ShakeProfiles {
		if p.Key == key {
			return p, nil
		}
	}
	return ShakeProfile{}, fmt.Errorf("%w: %q", ErrUnknownShake, key)
}

// Shake is a shake in rotor terms, as the hardware runs it.
type Shake struct {
	AmplitudeDeg float64 // rotor degrees each stroke
	Cycles       int     // forward-and-back pairs
	Rpm          float64 // rotor
	AccelRpmS    float64 // rotor
}

// profileMicrosteps is the microstepping Sorter V2's profile speeds were set at.
const profileMicrosteps = 16

// rotorShake turns a profile into rotor terms for a channel with this gearing.
func rotorShake(p ShakeProfile, gear float64) Shake {
	if gear <= 0 {
		gear = 1
	}
	stepsPerMotorRev := float64(fullStepsPerRev * profileMicrosteps)
	motorRpm := p.Speed / stepsPerMotorRev * 60
	motorAccel := p.Acceleration / stepsPerMotorRev * 60
	return Shake{AmplitudeDeg: p.AmplitudeDeg / gear, Cycles: p.Cycles, Rpm: motorRpm / gear, AccelRpmS: motorAccel / gear}
}

// shakeDuration estimates a shake: two ramped strokes per cycle.
func shakeDuration(s Shake) time.Duration {
	stroke := moveDuration(s.AmplitudeDeg, s.Rpm, s.AccelRpmS)
	return time.Duration(2*s.Cycles) * (stroke + 2*time.Millisecond)
}

func channelGear(settings MachineSettings, ch Channel) float64 {
	if ch == ChannelDistributor {
		return settings.Distributor.Gear
	}
	return settings.Channels[string(ch)].Gear
}

// RecoveryMove is one step of a procedure.
type RecoveryMove struct {
	Channel string  // "this" (the piece's channel) or "above" (the one feeding it)
	TurnDeg float64 // rotor degrees; negative turns back
	Shake   string  // a shake profile key, instead of turning
	// ProbeDeg turns this channel afterwards to see if the piece rides it (it moved
	// with the channel, so it's free). Probes turn back, never pushing toward an exit.
	ProbeDeg float64
	Settle   time.Duration
}

// Procedure is an ordered list of moves.
type Procedure struct {
	Key   string
	Title string
	Moves []RecoveryMove
}

// LandingHeld frees a piece where pieces land that its own channel doesn't carry: it
// most likely still hangs off the channel above.
var LandingHeld = Procedure{
	Key:   "landing_held",
	Title: "Free a piece held where pieces land",
	Moves: []RecoveryMove{
		{Channel: "above", TurnDeg: 3, ProbeDeg: -6},
		{Channel: "above", TurnDeg: 6, ProbeDeg: -6},
		{Channel: "above", TurnDeg: -6, ProbeDeg: -6},
		{Channel: "above", Shake: "brief", ProbeDeg: -6},
		{Channel: "this", Shake: "heavy", ProbeDeg: -6},
		{Channel: "this", TurnDeg: -15, ProbeDeg: -6},
		{Channel: "this", Shake: "hard", ProbeDeg: -6},
	},
}

const (
	recoverySettle = 500 * time.Millisecond
	recoveryRpm    = 5.0
)

// above is the channel that feeds ch.
func above(ch Channel) Channel {
	switch ch {
	case ChannelC3:
		return ChannelC2
	case ChannelC4:
		return ChannelC3
	default:
		return ChannelC1
	}
}

// recoveryRunner runs procedures with the hardware and cameras.
type recoveryRunner struct {
	hw       Hardware
	vision   VisionSource
	settings func() MachineSettings
}

// run tries a procedure's moves in order on a piece on ch, and reports whether a
// probe saw it ride free.
func (r recoveryRunner) run(ctx context.Context, proc Procedure, ch Channel, report reportFunc) (bool, error) {
	for i, mv := range proc.Moves {
		target := ch
		if mv.Channel == "above" {
			target = above(ch)
		}
		report(i+1, len(proc.Moves), r.describe(mv, target))
		if err := r.move(ctx, target, mv); err != nil {
			return false, err
		}
		if err := waitFor(ctx, max(mv.Settle, recoverySettle)); err != nil {
			return false, err
		}
		if mv.ProbeDeg == 0 {
			continue
		}
		free, err := r.probe(ctx, ch, mv.ProbeDeg)
		if err != nil {
			return false, err
		}
		if free {
			return true, nil
		}
	}
	return false, nil
}

func (r recoveryRunner) describe(mv RecoveryMove, ch Channel) string {
	if mv.Shake != "" {
		p, _ := shakeProfile(mv.Shake)
		return fmt.Sprintf("Shake %s: %s", ch, p.Label)
	}
	way := "forward"
	if mv.TurnDeg < 0 {
		way = "back"
	}
	return fmt.Sprintf("Turn %s %s %g°", ch, way, math.Abs(mv.TurnDeg))
}

func (r recoveryRunner) move(ctx context.Context, ch Channel, mv RecoveryMove) error {
	if mv.Shake != "" {
		return r.shake(ctx, ch, mv.Shake)
	}
	return r.turn(ctx, ch, mv.TurnDeg)
}

func (r recoveryRunner) turn(ctx context.Context, ch Channel, deg float64) error {
	done, err := r.hw.MoveChannel(ch, deg, recoveryRpm)
	if err != nil {
		return err
	}
	return awaitDone(ctx, done)
}

func (r recoveryRunner) shake(ctx context.Context, ch Channel, key string) error {
	p, err := shakeProfile(key)
	if err != nil {
		return err
	}
	done, err := r.hw.Shake(ch, rotorShake(p, channelGear(r.settings(), ch)))
	if err != nil {
		return err
	}
	return awaitDone(ctx, done)
}

// probe turns ch by deg and reports whether its leading piece moved with it.
func (r recoveryRunner) probe(ctx context.Context, ch Channel, deg float64) (bool, error) {
	before, ok := r.freshLead(ctx, ch, time.Now())
	if !ok {
		return true, nil // the channel is clear: whatever was held has gone
	}
	if err := r.turn(ctx, ch, deg); err != nil {
		return false, err
	}
	if err := waitFor(ctx, recoverySettle); err != nil {
		return false, err
	}
	after, ok := r.freshLead(ctx, ch, time.Now())
	if !ok {
		return true, nil
	}
	moved := math.Abs(after.ForwardToExit - before.ForwardToExit)
	return moved >= math.Abs(deg)/2, nil
}

// freshLead waits briefly for a frame taken after since and returns its leading piece.
func (r recoveryRunner) freshLead(ctx context.Context, ch Channel, since time.Time) (vision.PieceObs, bool) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		obs := r.vision.Observation(ch)
		if obs.Valid && !obs.At.Before(since) {
			return obs.Leading()
		}
		if waitFor(ctx, 20*time.Millisecond) != nil {
			break
		}
	}
	return r.vision.Observation(ch).Leading()
}

func awaitDone(ctx context.Context, done <-chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
