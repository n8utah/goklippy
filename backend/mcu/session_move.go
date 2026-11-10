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
