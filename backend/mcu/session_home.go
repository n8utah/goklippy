package mcu

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	// Klipper samples the endstop this often while homing and needs this many
	// matching samples in a row to call it triggered (~60 µs of debounce).
	homeSampleSec   = 15e-6
	homeSampleCount = 4
	// homeRestSec is the wait between sample bursts while the switch reads open.
	homeRestSec = 0.0005
	// homeTriggerReason is the trsync reason an endstop hit reports (Klipper's
	// REASON_ENDSTOP_HIT).
	homeTriggerReason = 1
	homePollEvery     = 20 * time.Millisecond
	homeSettle        = 50 * time.Millisecond
)

// ErrEndstopNotHit means a homing move ran its full length without the switch firing.
var ErrEndstopNotHit = errors.New("the endstop did not trigger")

// HomeMove drives one stepper toward its endstop until the switch fires.
type HomeMove struct {
	StepperOID int
	EndstopOID int
	Forward    bool // motor direction toward the switch
	SpeedHz    float64
	MaxSteps   int  // give up after this many steps
	Invert     bool // the switch reads low when triggered
}

// HomeResult is where the stepper was before homing and where the switch stopped it.
type HomeResult struct {
	Before int64
	After  int64
}

// EndstopTriggered reads an endstop now. invert is the endstop's "invert" meta.
func (s *Session) EndstopTriggered(endstopOID int, invert bool) (bool, error) {
	pin, _, err := s.endstopState(endstopOID)
	if err != nil {
		return false, err
	}
	return pin != invert, nil
}

// endstopState returns the pin reading and whether the endstop is still armed.
func (s *Session) endstopState(endstopOID int) (pinHigh, homing bool, err error) {
	params, err := s.Conn.QueryCommand(fmt.Sprintf("endstop_query_state oid=%d", endstopOID), "endstop_state")
	if err != nil {
		return false, false, err
	}
	pin, _ := params["pin_value"].(int64)
	armed, _ := params["homing"].(int64)
	return pin != 0, armed != 0, nil
}

// homeCommands arms the endstop on the stepper's stop trsync and queues the move. When
// the switch fires, the trsync stops the stepper on the MCU, with no host round trip.
func (s *Session) homeCommands(m HomeMove, stopTrsyncOID int, startClock uint64) []string {
	pinValue, dir := 1, 0
	if m.Invert {
		pinValue = 0
	}
	if m.Forward {
		dir = 1
	}
	interval := max(1, int(s.ClockFreq/clampJogSpeed(m.SpeedHz)))
	cmds := s.stepperStartCommands(m.StepperOID, stopTrsyncOID, startClock)
	cmds = append(cmds,
		fmt.Sprintf("endstop_home oid=%d clock=%d sample_ticks=%d sample_count=%d rest_ticks=%d pin_value=%d trsync_oid=%d trigger_reason=%d",
			m.EndstopOID, startClock, max(1, int(s.ClockFreq*homeSampleSec)), homeSampleCount,
			max(1, int(s.ClockFreq*homeRestSec)), pinValue, stopTrsyncOID, homeTriggerReason),
		fmt.Sprintf("set_next_step_dir oid=%d dir=%d", m.StepperOID, dir),
	)
	return append(cmds, queueStepCommands(m.StepperOID, interval, m.MaxSteps)...)
}

// disarmEndstopCommand stops an endstop from sampling.
func disarmEndstopCommand(endstopOID int) string {
	return fmt.Sprintf("endstop_home oid=%d clock=0 sample_ticks=0 sample_count=0 rest_ticks=0 pin_value=0 trsync_oid=0 trigger_reason=0", endstopOID)
}

// HomeStepper runs a homing move and waits for the switch. It replaces whatever the
// stepper was doing and holds the stepper for the whole move.
func (s *Session) HomeStepper(ctx context.Context, m HomeMove) (HomeResult, error) {
	if m.MaxSteps <= 0 || m.SpeedHz <= 0 {
		return HomeResult{}, fmt.Errorf("homing needs a positive speed and step limit")
	}
	s.stepperLock(m.StepperOID).Lock()
	defer s.stepperLock(m.StepperOID).Unlock()

	if err := s.ensureMCUReady(); err != nil {
		return HomeResult{}, err
	}
	s.stopStepperRoutineLocked(m.StepperOID)
	s.stopJogKeeper(m.StepperOID)
	s.setStepperJogging(m.StepperOID, false)

	stopTrsyncOID, err := s.stopTrsyncForStepper(m.StepperOID)
	if err != nil {
		return HomeResult{}, err
	}
	before, err := s.StepperPosition(m.StepperOID)
	if err != nil {
		return HomeResult{}, fmt.Errorf("read position: %w", err)
	}

	err = s.withRoutineSerial(func() error {
		_, err := s.sendAtMotionClock(motionStartMarginSec, func(startClock uint64) []string {
			return s.homeCommands(m, stopTrsyncOID, startClock)
		})
		return err
	})
	if err != nil {
		return HomeResult{}, err
	}
	if err := s.failIfShutdown(); err != nil {
		return HomeResult{}, err
	}
	slog.Info("stepper homing", "port", s.Port, "oid", m.StepperOID, "endstop", m.EndstopOID,
		"forward", m.Forward, "speed_hz", m.SpeedHz, "max_steps", m.MaxSteps)

	moveFor := time.Duration((motionStartMarginSec + float64(m.MaxSteps)/clampJogSpeed(m.SpeedHz)) * float64(time.Second))
	deadline := time.Now().Add(moveFor + 500*time.Millisecond)
	ticker := time.NewTicker(homePollEvery)
	defer ticker.Stop()

	abort := func(reason error) (HomeResult, error) {
		cmds := []string{disarmEndstopCommand(m.EndstopOID), fmt.Sprintf("trsync_trigger oid=%d reason=1", stopTrsyncOID)}
		if err := s.Conn.SendCommands(cmds); err != nil {
			slog.Warn("stopping a homing move failed", "port", s.Port, "oid", m.StepperOID, "error", err)
		}
		return HomeResult{Before: before}, reason
	}

	for {
		select {
		case <-ctx.Done():
			return abort(ctx.Err())
		case <-ticker.C:
		}
		_, armed, err := s.endstopState(m.EndstopOID)
		if err != nil {
			return abort(fmt.Errorf("read endstop: %w", err))
		}
		if !armed {
			break
		}
		if time.Now().After(deadline) {
			return abort(ErrEndstopNotHit)
		}
	}

	time.Sleep(homeSettle) // let the last step land before reading the position
	after, err := s.StepperPosition(m.StepperOID)
	if err != nil {
		return HomeResult{Before: before}, fmt.Errorf("read position: %w", err)
	}
	slog.Info("stepper homed", "port", s.Port, "oid", m.StepperOID, "moved_steps", after-before)
	return HomeResult{Before: before, After: after}, nil
}
