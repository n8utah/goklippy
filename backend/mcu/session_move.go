package mcu

import (
	"fmt"
	"log/slog"
	"time"
)

// StepperMove runs one accelerated point-to-point move. Like a fixed-step jog, it
// replaces whatever the stepper was doing. It returns how long from now until the last
// step fires, so the caller knows when the move is done.
func (s *Session) StepperMove(oid int, forward bool, move TrapezoidMove) (time.Duration, error) {
	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()

	if err := s.ensureMCUReady(); err != nil {
		return 0, err
	}
	s.stopStepperRoutineLocked(oid)
	s.stopJogKeeper(oid)
	s.setStepperJogging(oid, false)
	var d time.Duration
	err := s.withRoutineSerial(func() error {
		var err error
		d, err = s.stepperTrapezoidMove(oid, forward, move)
		return err
	})
	return d, err
}

// MaxShakeQueueOps caps one StepperShake call's queue_step commands, like a move, so
// it fits the MCU's move pool. Longer shakes go out in parts.
const MaxShakeQueueOps = maxMoveQueueOps

// StepperShake runs strokes alternating in direction, starting forward, each the
// same trapezoid move, queued as one sequence so they follow each other with no gap.
// It ends where it started when strokes is even. It returns how long it takes.
func (s *Session) StepperShake(oid int, forward bool, stroke TrapezoidMove, strokes int) (time.Duration, error) {
	if strokes < 1 {
		return 0, fmt.Errorf("a shake needs at least one stroke")
	}
	steps, err := moveQueueSteps(stroke, s.ClockFreq)
	if err != nil {
		return 0, err
	}
	if len(steps)*strokes > MaxShakeQueueOps {
		return 0, fmt.Errorf("that shake needs %d step commands, more than %d; use fewer strokes", len(steps)*strokes, MaxShakeQueueOps)
	}

	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()
	if err := s.ensureMCUReady(); err != nil {
		return 0, err
	}
	s.stopStepperRoutineLocked(oid)
	s.stopJogKeeper(oid)
	s.setStepperJogging(oid, false)
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return 0, err
	}

	// Every stroke: its direction, then its steps.
	var cmds []string
	for i := 0; i < strokes; i++ {
		dir := 0
		if (i%2 == 0) == forward {
			dir = 1
		}
		cmds = append(cmds, fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir))
		cmds = append(cmds, queueStepCommandsFor(oid, steps)...)
	}
	first := cmds[:min(len(cmds), maxQueueOpsPerBatch)]
	rest := cmds[len(first):]

	err = s.withRoutineSerial(func() error {
		if _, err := s.sendAtMotionClock(motionStartMarginSec, func(startClock uint64) []string {
			return append(s.stepperStartCommands(oid, stopTrsyncOID, startClock), first...)
		}); err != nil {
			return err
		}
		for len(rest) > 0 {
			batch := rest[:min(len(rest), maxQueueOpsPerBatch)]
			rest = rest[len(batch):]
			if err := s.Conn.SendCommands(batch); err != nil {
				return fmt.Errorf("send shake steps: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := s.failIfShutdown(); err != nil {
		return 0, err
	}
	sec := float64(queueStepsTicks(steps)) / s.ClockFreq * float64(strokes)
	slog.Info("stepper shake scheduled", "port", s.Port, "oid", oid, "strokes", strokes, "steps", stroke.Steps, "ms", int(sec*1000))
	return time.Duration((motionStartMarginSec + sec) * float64(time.Second)), nil
}

// stepperTrapezoidMove sends the move. The caller holds the stepper lock and the
// routine serial lock, so no other stepper's batch lands between ours. The first
// batch of queue_step commands is scheduled with reset_step_clock; the rest follow
// straight after in batches, while the first batch is still stepping out, so one move
// never has to fit the serial line inside the scheduling margin.
func (s *Session) stepperTrapezoidMove(oid int, forward bool, move TrapezoidMove) (time.Duration, error) {
	steps, err := moveQueueSteps(move, s.ClockFreq)
	if err != nil {
		return 0, err
	}
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return 0, err
	}
	dir := 0
	if forward {
		dir = 1
	}
	first := steps[:min(len(steps), maxQueueOpsPerBatch)]
	rest := steps[len(first):]

	_, err = s.sendAtMotionClock(motionStartMarginSec, func(startClock uint64) []string {
		cmds := s.stepperStartCommands(oid, stopTrsyncOID, startClock)
		cmds = append(cmds, fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir))
		return append(cmds, queueStepCommandsFor(oid, first)...)
	})
	if err != nil {
		return 0, err
	}
	for len(rest) > 0 {
		batch := rest[:min(len(rest), maxQueueOpsPerBatch)]
		rest = rest[len(batch):]
		if err := s.Conn.SendCommands(queueStepCommandsFor(oid, batch)); err != nil {
			return 0, fmt.Errorf("send move steps: %w", err)
		}
	}
	if err := s.failIfShutdown(); err != nil {
		return 0, err
	}

	moveSec := float64(queueStepsTicks(steps)) / s.ClockFreq
	slog.Info("stepper move scheduled",
		"port", s.Port,
		"oid", oid,
		"forward", forward,
		"steps", move.Steps,
		"max_hz", move.MaxHz,
		"accel_hz2", move.AccelHz,
		"queue_ops", len(steps),
		"move_ms", int(moveSec*1000),
	)
	return time.Duration((motionStartMarginSec + moveSec) * float64(time.Second)), nil
}

// ShakeQueueOps is how many queue_step commands a shake needs, for checking a shake
// fits before sending it.
func ShakeQueueOps(stroke TrapezoidMove, strokes int, clockFreq float64) (int, error) {
	steps, err := moveQueueSteps(stroke, clockFreq)
	if err != nil {
		return 0, err
	}
	return len(steps) * strokes, nil
}
