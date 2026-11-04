package mcu

import (
	"fmt"
	"log/slog"
	"time"
)

// StepperRoutineSegment is one forward/backward move in a repeating pattern.
type StepperRoutineSegment struct {
	Forward bool
	SpeedHz float64
	Steps   int
}

type stepperRoutineKeeper struct {
	segments []StepperRoutineSegment
	stop     chan struct{}
}

const (
	routineScheduleMarginSec = 0.02
	routineRefillLeadRatio   = 0.35
	routineRefillReps        = 2
	// maxQueueOpsPerBatch limits queue_step commands per serial batch. The Klipper MCU
	// move pool (1024) is shared across all steppers — large batches cause shutdown.
	maxQueueOpsPerBatch        = 40
	maxPatternBatchRepetitions = 12
	minQueueOpsPerBatch        = 6
)

func patternMoveDuration(segments []StepperRoutineSegment) time.Duration {
	var moveSec float64
	for _, seg := range segments {
		speedHz := clampJogSpeed(seg.SpeedHz)
		if seg.Steps <= 0 || speedHz <= 0 {
			continue
		}
		moveSec += float64(seg.Steps) / speedHz
	}
	if moveSec <= 0 {
		return 50 * time.Millisecond
	}
	return time.Duration(moveSec * float64(time.Second))
}

func patternDuration(segments []StepperRoutineSegment) time.Duration {
	return patternMoveDuration(segments) + time.Duration(0.028*float64(time.Second))
}

func patternQueueStepCount(segments []StepperRoutineSegment) int {
	count := 0
	for _, seg := range segments {
		if seg.Steps <= 0 {
			continue
		}
		n := seg.Steps / maxQueueStepCount
		if seg.Steps%maxQueueStepCount != 0 {
			n++
		}
		count += n
	}
	if count < 1 {
		return 1
	}
	return count
}

func routineBatchQueueOps(segments []StepperRoutineSegment, reps int) int {
	return patternQueueStepCount(repeatPatternSegments(segments, reps))
}

func routineQueueOpsLimitFor(activeRoutines int) int {
	if activeRoutines < 1 {
		activeRoutines = 1
	}
	limit := maxQueueOpsPerBatch / activeRoutines
	if limit < minQueueOpsPerBatch {
		limit = minQueueOpsPerBatch
	}
	return limit
}

func (s *Session) withRoutineSerial(fn func() error) error {
	s.routineSerialMu.Lock()
	defer s.routineSerialMu.Unlock()
	return fn()
}

// patternBatchRepetitions is how many loop copies fit in one MCU batch.
func patternBatchRepetitions(segments []StepperRoutineSegment, opsLimit int) int {
	if opsLimit <= 0 {
		opsLimit = maxQueueOpsPerBatch
	}
	qsc := patternQueueStepCount(segments)
	if qsc <= 0 {
		return 1
	}
	max := opsLimit / qsc
	if max < 1 {
		max = 1
	}
	if max > maxPatternBatchRepetitions {
		max = maxPatternBatchRepetitions
	}
	return max
}

func patternRefillRepetitions(segments []StepperRoutineSegment, opsLimit int) int {
	batch := patternBatchRepetitions(segments, opsLimit)
	reps := routineRefillReps
	if batch/2 < reps {
		reps = batch / 2
	}
	if reps < 1 {
		reps = 1
	}
	return reps
}

func routineRefillWait(segments []StepperRoutineSegment, activeRoutines int) time.Duration {
	move := patternMoveDuration(segments)
	ratio := routineRefillLeadRatio
	if activeRoutines > 1 {
		ratio += 0.1 * float64(activeRoutines-1)
	}
	wait := time.Duration(float64(move) * ratio)
	min := 40 * time.Millisecond
	if wait < min {
		return min
	}
	return wait
}

func routineStartStagger(oid int) time.Duration {
	if oid <= 0 {
		return 0
	}
	return time.Duration(oid%4) * 35 * time.Millisecond
}

func repeatPatternSegments(segments []StepperRoutineSegment, reps int) []StepperRoutineSegment {
	if reps <= 1 {
		return segments
	}
	out := make([]StepperRoutineSegment, 0, len(segments)*reps)
	for i := 0; i < reps; i++ {
		out = append(out, segments...)
	}
	return out
}

func validateRoutineSegments(segments []StepperRoutineSegment) error {
	if len(segments) == 0 {
		return fmt.Errorf("routine requires at least one segment")
	}
	for i, seg := range segments {
		if seg.Steps <= 0 {
			return fmt.Errorf("segment %d: steps must be positive", i+1)
		}
		if clampJogSpeed(seg.SpeedHz) <= 0 {
			return fmt.Errorf("segment %d: speed_hz must be positive", i+1)
		}
	}
	return nil
}

// StartStepperRoutine runs a step pattern loop on one stepper until stopped.
func (s *Session) StartStepperRoutine(oid int, segments []StepperRoutineSegment) error {
	if err := validateRoutineSegments(segments); err != nil {
		return err
	}

	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()

	s.stopStepperRoutineLocked(oid)
	s.stopJogKeeper(oid)
	s.setStepperJogging(oid, false)

	segs := make([]StepperRoutineSegment, len(segments))
	copy(segs, segments)
	state := &stepperRoutineKeeper{
		segments: segs,
		stop:     make(chan struct{}),
	}
	s.routineKeepersMu.Lock()
	if s.routineKeepers == nil {
		s.routineKeepers = make(map[int]*stepperRoutineKeeper)
	}
	s.routineKeepers[oid] = state
	s.routineKeepersMu.Unlock()

	go s.stepperRoutineLoop(oid, state)
	return nil
}

func (s *Session) stopStepperRoutineLocked(oid int) {
	s.routineKeepersMu.Lock()
	state, ok := s.routineKeepers[oid]
	if ok {
		delete(s.routineKeepers, oid)
	}
	s.routineKeepersMu.Unlock()
	if !ok {
		return
	}
	select {
	case <-state.stop:
	default:
		close(state.stop)
	}
}

func (s *Session) stopAllStepperRoutines() {
	s.routineKeepersMu.Lock()
	keepers := s.routineKeepers
	s.routineKeepers = make(map[int]*stepperRoutineKeeper)
	s.routineKeepersMu.Unlock()
	for _, state := range keepers {
		select {
		case <-state.stop:
		default:
			close(state.stop)
		}
	}
}

func routineSegmentCommands(oid int, segments []StepperRoutineSegment, clockFreq float64) []string {
	cmds := make([]string, 0, len(segments)*2)
	for _, seg := range segments {
		speedHz := clampJogSpeed(seg.SpeedHz)
		dir := 0
		if seg.Forward {
			dir = 1
		}
		interval := int(clockFreq / speedHz)
		if interval < 1 {
			interval = 1
		}
		cmds = append(cmds, fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir))
		cmds = append(cmds, queueStepCommands(oid, interval, seg.Steps)...)
	}
	return cmds
}

func (s *Session) sendStepperRoutinePattern(oid int, segments []StepperRoutineSegment, fastClock bool) (uint64, error) {
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return 0, err
	}

	var startClock uint64
	if fastClock {
		startClock = s.scheduleClock(routineScheduleMarginSec)
	} else {
		startClock, err = s.motionClock(routineScheduleMarginSec)
		if err != nil {
			return 0, err
		}
	}

	cmds := make([]string, 0, 10+len(segments)*3)
	cmds = append(cmds, s.stepperDriverEnableCommands(oid)...)
	cmds = append(cmds, fmt.Sprintf("trsync_trigger oid=%d reason=1", stopTrsyncOID))
	cmds = append(cmds,
		fmt.Sprintf("trsync_start oid=%d report_clock=0 report_ticks=0 expire_reason=0", stopTrsyncOID),
		fmt.Sprintf("stepper_stop_on_trigger oid=%d trsync_oid=%d", oid, stopTrsyncOID),
		fmt.Sprintf("reset_step_clock oid=%d clock=%d", oid, startClock),
	)
	cmds = append(cmds, routineSegmentCommands(oid, segments, s.ClockFreq)...)

	if err := s.Conn.SendCommands(cmds); err != nil {
		return 0, err
	}
	return startClock, nil
}

func (s *Session) stepperRoutinePatternMove(oid int, segments []StepperRoutineSegment, fastClock bool) error {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Info("retrying stepper routine pattern",
				"port", s.Port,
				"oid", oid,
				"attempt", attempt+1,
			)
			if err := s.recoverMCUShutdown(); err != nil {
				return err
			}
		}

		startClock, err := s.sendStepperRoutinePattern(oid, segments, fastClock)
		if err != nil {
			return err
		}

		slog.Info("stepper routine pattern scheduled",
			"port", s.Port,
			"oid", oid,
			"segments", len(segments),
			"queue_ops", patternQueueStepCount(segments),
			"start_clock", startClock,
			"fast_clock", fastClock,
		)
		return nil
	}
	return fmt.Errorf("stepper routine pattern failed after retry")
}

// stepperRoutineAppend queues more pattern steps without reset_step_clock.
// Only works while the MCU stepper queue is still draining.
func (s *Session) stepperRoutineAppend(oid int, segments []StepperRoutineSegment, opsLimit int) error {
	if ops := patternQueueStepCount(segments); ops > opsLimit {
		return fmt.Errorf("append batch too large: %d queue ops (max %d)", ops, opsLimit)
	}
	cmds := routineSegmentCommands(oid, segments, s.ClockFreq)
	return s.Conn.SendCommands(cmds)
}

func (s *Session) stepperRoutineLoop(oid int, state *stepperRoutineKeeper) {
	defer func() {
		s.routineKeepersMu.Lock()
		delete(s.routineKeepers, oid)
		s.routineKeepersMu.Unlock()
	}()

	time.Sleep(routineStartStagger(oid))

	moveDur := patternMoveDuration(state.segments)
	activeRoutines := s.activeRoutineCount()
	opsLimit := routineQueueOpsLimitFor(activeRoutines)
	batchReps := patternBatchRepetitions(state.segments, opsLimit)
	refillReps := patternRefillRepetitions(state.segments, opsLimit)
	refillWait := routineRefillWait(state.segments, activeRoutines)

	scheduleBatch := func(reps int) time.Time {
		return time.Now().Add(time.Duration(reps) * moveDur)
	}

	batchSegments := repeatPatternSegments(state.segments, batchReps)
	err := s.withRoutineSerial(func() error {
		return s.stepperRoutinePatternMove(oid, batchSegments, false)
	})
	if err != nil {
		slog.Warn("stepper routine pattern failed",
			"port", s.Port,
			"oid", oid,
			"error", err,
		)
		return
	}
	queuedUntil := scheduleBatch(batchReps)

	for {
		timer := time.NewTimer(refillWait)
		select {
		case <-state.stop:
			timer.Stop()
			s.stepperLock(oid).Lock()
			_ = s.stepperStopUnlocked(oid)
			s.stepperLock(oid).Unlock()
			return
		case <-timer.C:
		}

		opsLimit = routineQueueOpsLimitFor(s.activeRoutineCount())
		activeRoutines := s.activeRoutineCount()
		batchReps = patternBatchRepetitions(state.segments, opsLimit)
		refillReps = patternRefillRepetitions(state.segments, opsLimit)
		refillWait = routineRefillWait(state.segments, activeRoutines)

		remaining := time.Until(queuedUntil)
		lowWater := moveDur / 2
		topUpBelow := moveDur * 2

		var refillErr error
		refillErr = s.withRoutineSerial(func() error {
			if remaining <= lowWater {
				batchSegments = repeatPatternSegments(state.segments, batchReps)
				if err := s.stepperRoutinePatternMove(oid, batchSegments, true); err != nil {
					return err
				}
				queuedUntil = scheduleBatch(batchReps)
				slog.Info("stepper routine batch restarted",
					"port", s.Port,
					"oid", oid,
					"reps", batchReps,
					"queue_ops", patternQueueStepCount(batchSegments),
				)
				return nil
			}
			if remaining <= topUpBelow {
				refillSegments := repeatPatternSegments(state.segments, refillReps)
				if err := s.stepperRoutineAppend(oid, refillSegments, opsLimit); err != nil {
					return err
				}
				queuedUntil = queuedUntil.Add(time.Duration(refillReps) * moveDur)
			}
			return nil
		})

		if refillErr != nil {
			slog.Warn("stepper routine refill failed, will retry",
				"port", s.Port,
				"oid", oid,
				"error", refillErr,
			)
		}
	}
}

func (s *Session) activeRoutineCount() int {
	s.routineKeepersMu.Lock()
	defer s.routineKeepersMu.Unlock()
	return len(s.routineKeepers)
}

func (s *Session) stepperStopUnlocked(oid int) error {
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return err
	}
	s.stopJogKeeper(oid)
	s.setStepperJogging(oid, false)
	return s.Conn.SendCommand(fmt.Sprintf("trsync_trigger oid=%d reason=1", stopTrsyncOID))
}
