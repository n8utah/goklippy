package mcu

import (
	"fmt"
	"log/slog"
	"math"
	"time"
)

// indexSettleMargin separates one indexed move's last step from the next move's start.
const indexSettleMargin = motionStartMarginSec + 0.01

// IndexMove rotates a stepper in fixed increments with a pause between them.
type IndexMove struct {
	Forward       bool
	SpeedHz       float64
	StepsPerIndex float64 // may be fractional; the remainder carries to the next move
	AccelHz       float64 // steps per second²; 0 moves at SpeedHz with no ramp
	Dwell         time.Duration
	Count         int // 0 repeats until stopped
}

// StartStepperIndexing runs an IndexMove. It shares the routine bookkeeping, so a stop,
// jog or new routine on this stepper cancels it and the UI sees it as running.
func (s *Session) StartStepperIndexing(oid int, move IndexMove) error {
	if move.StepsPerIndex < 1 {
		return fmt.Errorf("steps per index must be at least 1")
	}
	if move.Count < 0 || move.Dwell < 0 || move.AccelHz < 0 {
		return fmt.Errorf("count, dwell and acceleration can't be negative")
	}

	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()

	if err := s.ensureMCUReady(); err != nil {
		return err
	}
	s.stopStepperRoutineLocked(oid)
	s.stopJogKeeper(oid)
	s.setStepperJogging(oid, false)

	state := &stepperRoutineKeeper{stop: make(chan struct{})}
	s.routineKeepersMu.Lock()
	if s.routineKeepers == nil {
		s.routineKeepers = make(map[int]*stepperRoutineKeeper)
	}
	s.routineKeepers[oid] = state
	s.routineKeepersMu.Unlock()

	go s.indexLoop(oid, move, state)
	return nil
}

// nextIndexSteps is the whole-step size of the next move. Keeping the rounding error
// between moves means fractional gearing can't drift the carousel off its slots.
func (s *Session) nextIndexSteps(oid int, forward bool, stepsPerIndex float64) int {
	s.routineKeepersMu.Lock()
	defer s.routineKeepersMu.Unlock()
	if s.indexRemainders == nil {
		s.indexRemainders = make(map[int]float64)
	}
	signed := stepsPerIndex
	if !forward {
		signed = -signed
	}
	wanted := s.indexRemainders[oid] + signed
	steps := math.Round(wanted)
	s.indexRemainders[oid] = wanted - steps
	return int(math.Abs(steps))
}

func (s *Session) indexLoop(oid int, move IndexMove, state *stepperRoutineKeeper) {
	defer func() {
		s.routineKeepersMu.Lock()
		if s.routineKeepers[oid] == state {
			delete(s.routineKeepers, oid)
		}
		s.routineKeepersMu.Unlock()
	}()

	for i := 0; move.Count == 0 || i < move.Count; i++ {
		steps := s.nextIndexSteps(oid, move.Forward, move.StepsPerIndex)
		moveDur := time.Duration((float64(steps)/clampJogSpeed(move.SpeedHz) + indexSettleMargin) * float64(time.Second))
		err := s.sendIfActive(oid, state, func() error {
			if move.AccelHz <= 0 {
				return s.stepperJogMove(oid, move.Forward, move.SpeedHz, steps)
			}
			d, err := s.stepperTrapezoidMove(oid, move.Forward, TrapezoidMove{
				Steps: steps, MaxHz: move.SpeedHz, AccelHz: move.AccelHz,
			})
			// d already counts the scheduling margin; add the same settle time.
			moveDur = d + time.Duration((indexSettleMargin-motionStartMarginSec)*float64(time.Second))
			return err
		})
		if err == errRoutineStopped {
			return
		}
		if err != nil {
			slog.Warn("stepper indexing failed", "port", s.Port, "oid", oid, "error", err)
			s.failRoutine(oid, state, err)
			return
		}

		timer := time.NewTimer(moveDur + move.Dwell)
		select {
		case <-state.stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
