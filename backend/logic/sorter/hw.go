package sorter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrInvalidSettings   = errors.New("invalid sorter settings")
	ErrInvalidTransition = errors.New("not allowed in the sorter's current state")
	ErrNoBoard           = errors.New("no board connected; connect one under Machine › Board")
	ErrUnknownChannel    = errors.New("unknown channel")
)

// Channel names a rotating part of the machine.
type Channel string

const (
	ChannelC1          Channel = "c1"
	ChannelC2          Channel = "c2"
	ChannelC3          Channel = "c3"
	ChannelC4          Channel = "c4"
	ChannelDistributor Channel = "distributor"
)

// Channels lists every channel the sorter drives.
var Channels = []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4, ChannelDistributor}

// ParseChannel checks a channel name from a request.
func ParseChannel(name string) (Channel, error) {
	for _, ch := range Channels {
		if string(ch) == name {
			return ch, nil
		}
	}
	return "", ErrUnknownChannel
}

// Hardware is everything the sorter does to the machine, in rotor units. Positive
// degrees turn a channel in its forward direction (the way pieces travel).
type Hardware interface {
	// MoveChannel turns a channel by deg at up to rpm, ramping by the channel's
	// acceleration setting. The returned channel receives once when the move should
	// have finished.
	MoveChannel(ch Channel, deg, rpm float64) (<-chan error, error)
	// SelectLayer sends the next piece to one distribution layer: its servo goes to
	// the open angle and the rest to closed. -1 selects none, so every layer is closed
	// and pieces fall through to the bottom bucket.
	SelectLayer(layer int) error
	// Shake runs a channel back and forth, ending where it began.
	Shake(ch Channel, s Shake) (<-chan error, error)
	// StopAll stops every sorter motor.
	StopAll() error
	Health() HardwareHealth

	// HomeChute turns the chute counter-clockwise onto its home switch, which becomes
	// 0°. It blocks until the switch fires or the move gives up.
	HomeChute(ctx context.Context) error
	// ZeroChute makes wherever the chute points now 0°, without the switch. A
	// fallback for a machine with no switch fitted.
	ZeroChute() error
	// AimChute turns the homed chute to an angle, clockwise from home, within its
	// travel. The returned channel receives once when the move should be done.
	AimChute(angle float64) (<-chan error, error)
	ChuteStatus() ChuteStatus

	// SetLights turns the machine's light on or off.
	SetLights(on bool) error
	LightsStatus() LightsStatus

	// SupplyVoltage is the motors' supply, if it's measured.
	SupplyVoltage() SupplyStatus
}

// SupplyStatus is the motors' supply voltage. Known is false when it isn't measured
// (no sensor in the board config, or no reading yet); then nothing is held back.
type SupplyStatus struct {
	Known    bool    `json:"known"`
	Volts    float64 `json:"volts,omitempty"`
	MinVolts float64 `json:"min_volts,omitempty"`
	Low      bool    `json:"low,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// ErrSupplyLow refuses to home or sort without motor power.
var ErrSupplyLow = errors.New("the motors' supply is low")

func (s SupplyStatus) lowError() error {
	return fmt.Errorf("%w: %.1f V, needs %.1f V. Check the 12 V supply to the RAMPS 5 A input", ErrSupplyLow, s.Volts, s.MinVolts)
}

func (a SupplyStatus) same(b SupplyStatus) bool {
	// Small changes in the voltage aren't news.
	return a.Known == b.Known && a.Low == b.Low && a.Message == b.Message && math.Abs(a.Volts-b.Volts) < 0.1
}

// LightsStatus is whether the light is on. Message says why it can't be switched
// (no board, or its output isn't in the board config).
type LightsStatus struct {
	On      bool   `json:"on"`
	Message string `json:"message,omitempty"`
}

// ChuteStatus is where the chute is and what its switch reads.
type ChuteStatus struct {
	Homed bool     `json:"homed"`
	Angle *float64 `json:"angle,omitempty"` // only once homed
	// Endstop is "triggered", "open", or "" when there's no switch to read.
	Endstop string `json:"endstop,omitempty"`
	Message string `json:"message,omitempty"`
}

var ErrChuteNotHomed = errors.New("the chute isn't homed; home it first")

var ErrNoLight = errors.New("the light's output is not in the board config")

// HardwareHealth is what the UI shows about the hardware.
type HardwareHealth struct {
	Connected bool   `json:"connected"`
	Simulated bool   `json:"simulated,omitempty"`
	Port      string `json:"port,omitempty"`
	Message   string `json:"message,omitempty"`
}

// channelAccel is the channel's acceleration in rpm per second (0 = no ramp).
func channelAccel(settings MachineSettings, ch Channel) float64 {
	switch ch {
	case ChannelC4:
		return settings.IndexAccel
	case ChannelDistributor:
		return settings.Distributor.Accel
	default:
		return 0
	}
}

// moveDuration estimates how long a rotor move takes with a trapezoid ramp.
func moveDuration(deg, rpm, accelRpmPerSec float64) time.Duration {
	dist := math.Abs(deg) / 360 // rotor turns
	speed := rpm / 60           // turns per second
	if dist == 0 || speed <= 0 {
		return 0
	}
	if accelRpmPerSec <= 0 {
		return seconds(dist / speed)
	}
	accel := accelRpmPerSec / 60 // turns per second²
	ramp := speed * speed / (2 * accel)
	if 2*ramp >= dist {
		return seconds(2 * math.Sqrt(dist/accel))
	}
	return seconds(2*speed/accel + (dist-2*ramp)/speed)
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// doneAfter returns a channel that receives nil after d.
func doneAfter(d time.Duration) <-chan error {
	done := make(chan error, 1)
	time.AfterFunc(d, func() { done <- nil })
	return done
}
