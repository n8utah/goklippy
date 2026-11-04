package mcu

import (
	"fmt"
	"log/slog"
	"sort"
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
	// routineDrainMarginSec: with less than this left in the MCU queue, an append could
	// arrive after the queue empties (the MCU shuts down on steps in the past), so the
	// loop lets the queue finish and starts a fresh batch instead.
	routineDrainMarginSec = 0.03
	routineClockResync    = 2 * time.Second
	// routineMaxRefillFailures in a row ends the routine and reports the error.
	routineMaxRefillFailures = 3
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

// stopStepperRoutineLocked cancels the oid's routine, if any, and clears its last
// failure: a stop, jog or new routine means the user has moved on from it.
func (s *Session) stopStepperRoutineLocked(oid int) {
	s.routineKeepersMu.Lock()
	state, ok := s.routineKeepers[oid]
	if ok {
		delete(s.routineKeepers, oid)
	}
	delete(s.routineErrors, oid)
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
	for oid, state := range keepers {
		s.stepperLock(oid).Lock()
		if !state.stopped() {
			close(state.stop)
			_ = s.stepperStopUnlocked(oid)
		}
		s.stepperLock(oid).Unlock()
	}
}

// RoutineState is one stepper's routine as the UI should show it.
type RoutineState struct {
	OID     int    `json:"oid"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// RoutineStates lists running routines and ones that stopped on an error, by oid.
func (s *Session) RoutineStates() []RoutineState {
	s.routineKeepersMu.Lock()
	defer s.routineKeepersMu.Unlock()
	states := make([]RoutineState, 0, len(s.routineKeepers)+len(s.routineErrors))
	for oid := range s.routineKeepers {
		states = append(states, RoutineState{OID: oid, Running: true})
	}
	for oid, msg := range s.routineErrors {
		if _, running := s.routineKeepers[oid]; !running {
			states = append(states, RoutineState{OID: oid, Error: msg})
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].OID < states[j].OID })
	return states
}

// failRoutine records why a routine gave up, unless a newer one replaced it.
func (s *Session) failRoutine(oid int, state *stepperRoutineKeeper, err error) {
	s.routineKeepersMu.Lock()
	defer s.routineKeepersMu.Unlock()
	if s.routineKeepers[oid] != state {
		return
	}
	if s.routineErrors == nil {
		s.routineErrors = make(map[int]string)
	}
	s.routineErrors[oid] = err.Error()
}

func routineStepInterval(clockFreq, speedHz float64) int {
	interval := int(clockFreq / clampJogSpeed(speedHz))
	if interval < 1 {
		interval = 1
	}
	return interval
}

// patternTicks is how many MCU clock ticks the segments take to step out, matching
// the intervals routineSegmentCommands sends.
func patternTicks(segments []StepperRoutineSegment, clockFreq float64) uint64 {
	var ticks uint64
	for _, seg := range segments {
		if seg.Steps > 0 {
			ticks += uint64(seg.Steps) * uint64(routineStepInterval(clockFreq, seg.SpeedHz))
		}
	}
	return ticks
}

// clockAhead is how far b is past a on the MCU's 32-bit clock, negative if b is
// earlier. Valid while they are within half a wrap (~134 s at 16 MHz).
func clockAhead(a, b uint64) int64 {
	return int64(int32(uint32(b) - uint32(a)))
}

// routineClockNow is the MCU clock, re-read from the MCU when the host estimate is
// older than routineClockResync so crystal drift can't build up over a long run.
func (s *Session) routineClockNow() uint64 {
	s.mu.RLock()
	stale := time.Since(s.ClockBaseAt) > routineClockResync
	s.mu.RUnlock()
	if stale {
		if clock, err := s.motionClock(0); err == nil {
			return clock
		}
	}
	return s.EstimatedClock() & 0xffffffff
}

func routineSegmentCommands(oid int, segments []StepperRoutineSegment, clockFreq float64) []string {
	cmds := make([]string, 0, len(segments)*2)
	for _, seg := range segments {
		dir := 0
		if seg.Forward {
			dir = 1
		}
		interval := routineStepInterval(clockFreq, seg.SpeedHz)
		cmds = append(cmds, fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir))
		cmds = append(cmds, queueStepCommands(oid, interval, seg.Steps)...)
	}
	return cmds
}

func (s *Session) sendStepperRoutinePattern(oid int, segments []StepperRoutineSegment) (uint64, error) {
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return 0, err
	}

	// A live clock read: queueEnd is counted from this, so it has to be accurate.
	startClock, err := s.motionClock(routineScheduleMarginSec)
	if err != nil {
		return 0, err
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

// stepperRoutinePatternMove starts a fresh batch and returns the MCU clock it starts at.
func (s *Session) stepperRoutinePatternMove(oid int, segments []StepperRoutineSegment) (uint64, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Info("retrying stepper routine pattern",
				"port", s.Port,
				"oid", oid,
				"attempt", attempt+1,
			)
			if err := s.recoverMCUShutdown(); err != nil {
				return 0, err
			}
		}

		startClock, err := s.sendStepperRoutinePattern(oid, segments)
		if err != nil {
			// Retry once if the MCU shut down (e.g. a move scheduled in the past);
			// anything else is a real failure.
			if shutdown, qerr := s.mcuIsShutdown(); attempt == 0 && qerr == nil && shutdown {
				continue
			}
			return 0, err
		}

		slog.Info("stepper routine pattern scheduled",
			"port", s.Port,
			"oid", oid,
			"segments", len(segments),
			"queue_ops", patternQueueStepCount(segments),
			"start_clock", startClock,
		)
		return startClock, nil
	}
	return 0, fmt.Errorf("stepper routine pattern failed after retry")
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

// stopped reports whether the routine was cancelled.
func (state *stepperRoutineKeeper) stopped() bool {
	select {
	case <-state.stop:
		return true
	default:
		return false
	}
}

// errRoutineStopped ends the loop quietly when a stop, jog or new routine took over.
var errRoutineStopped = fmt.Errorf("routine stopped")

// sendIfActive runs fn under the stepper lock unless the routine was cancelled. Whoever
// cancels a routine (StepperStop, StepperJog, StartStepperRoutine) holds that lock while
// closing stop, so once the loop sees stop closed it must not touch the stepper: the new
// owner's commands are already queued.
func (s *Session) sendIfActive(oid int, state *stepperRoutineKeeper, fn func() error) error {
	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()
	if state.stopped() {
		return errRoutineStopped
	}
	return s.withRoutineSerial(fn)
}

func (s *Session) stepperRoutineLoop(oid int, state *stepperRoutineKeeper) {
	defer func() {
		s.routineKeepersMu.Lock()
		// A newer routine on this oid may have replaced us; leave its entry alone.
		if s.routineKeepers[oid] == state {
			delete(s.routineKeepers, oid)
		}
		s.routineKeepersMu.Unlock()
	}()

	timer := time.NewTimer(routineStartStagger(oid))
	select {
	case <-state.stop:
		timer.Stop()
		return
	case <-timer.C:
	}

	moveDur := patternMoveDuration(state.segments)
	activeRoutines := s.activeRoutineCount()
	opsLimit := routineQueueOpsLimitFor(activeRoutines)
	batchReps := patternBatchRepetitions(state.segments, opsLimit)
	refillReps := patternRefillRepetitions(state.segments, opsLimit)
	refillWait := routineRefillWait(state.segments, activeRoutines)

	// queueEnd is the MCU clock at which the last queued step fires. Every batch and
	// top-up is whole pattern repetitions, so when the queue empties the stepper is
	// back at a pattern boundary and a fresh batch carries on without losing steps.
	var queueEnd uint64
	startBatch := func() error {
		batchSegments := repeatPatternSegments(state.segments, batchReps)
		return s.sendIfActive(oid, state, func() error {
			startClock, err := s.stepperRoutinePatternMove(oid, batchSegments)
			if err != nil {
				return err
			}
			queueEnd = (startClock + patternTicks(batchSegments, s.ClockFreq)) & 0xffffffff
			return nil
		})
	}
	// sleep waits d, returning false if the routine was cancelled meanwhile.
	sleep := func(d time.Duration) bool {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-state.stop:
			return false
		case <-timer.C:
			return true
		}
	}

	if err := startBatch(); err != nil {
		if err != errRoutineStopped {
			slog.Warn("stepper routine pattern failed",
				"port", s.Port,
				"oid", oid,
				"error", err,
			)
			s.failRoutine(oid, state, err)
		}
		return
	}

	failures := 0
	for {
		// No stop command on cancel: the canceller already stopped or replaced the motion.
		if !sleep(refillWait) {
			return
		}

		activeRoutines = s.activeRoutineCount()
		opsLimit = routineQueueOpsLimitFor(activeRoutines)
		batchReps = patternBatchRepetitions(state.segments, opsLimit)
		refillReps = patternRefillRepetitions(state.segments, opsLimit)
		refillWait = routineRefillWait(state.segments, activeRoutines)
		topUpBelow := moveDur * 2

		var drainFor time.Duration
		restart := false
		err := s.sendIfActive(oid, state, func() error {
			left := time.Duration(float64(clockAhead(s.routineClockNow(), queueEnd)) / s.ClockFreq * float64(time.Second))
			switch {
			case left <= time.Duration(routineDrainMarginSec*float64(time.Second)):
				// Too late to append safely. Let the queued steps finish rather than
				// cutting them off, then start over from the pattern's first segment.
				restart, drainFor = true, max(left, 0)
			case left <= topUpBelow:
				refillSegments := repeatPatternSegments(state.segments, refillReps)
				if err := s.stepperRoutineAppend(oid, refillSegments, opsLimit); err != nil {
					return err
				}
				queueEnd = (queueEnd + patternTicks(refillSegments, s.ClockFreq)) & 0xffffffff
			}
			return nil
		})
		if err == nil && restart {
			slog.Info("stepper routine queue ran low, restarting after it drains",
				"port", s.Port,
				"oid", oid,
				"drain_ms", drainFor.Milliseconds(),
			)
			if !sleep(drainFor + 5*time.Millisecond) {
				return
			}
			err = startBatch()
		}

		if err == errRoutineStopped {
			return
		}
		if err != nil {
			failures++
			slog.Warn("stepper routine refill failed",
				"port", s.Port,
				"oid", oid,
				"failures", failures,
				"error", err,
			)
			if failures >= routineMaxRefillFailures {
				s.failRoutine(oid, state, err)
				return
			}
			continue
		}
		failures = 0
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
