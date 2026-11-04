package mcu

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPatternDuration(t *testing.T) {
	segments := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 100},
		{Forward: false, SpeedHz: 500, Steps: 10},
	}
	d := patternDuration(segments)
	if d < 235*time.Millisecond || d > 260*time.Millisecond {
		t.Fatalf("expected ~248ms for chained 100+10 steps @ 500Hz, got %v", d)
	}
}

func TestPatternQueueStepCount(t *testing.T) {
	segments := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 100},
		{Forward: false, SpeedHz: 500, Steps: 10},
	}
	if got := patternQueueStepCount(segments); got != 2 {
		t.Fatalf("expected 2 queue_step ops, got %d", got)
	}
}

func TestRoutineQueueOpsLimitFor(t *testing.T) {
	if got := routineQueueOpsLimitFor(1); got != 40 {
		t.Fatalf("expected 40 for 1 routine, got %d", got)
	}
	if got := routineQueueOpsLimitFor(2); got != 20 {
		t.Fatalf("expected 20 for 2 routines, got %d", got)
	}
	if got := routineQueueOpsLimitFor(10); got != minQueueOpsPerBatch {
		t.Fatalf("expected floor %d for many routines, got %d", minQueueOpsPerBatch, got)
	}
}

func TestPatternBatchRepetitions(t *testing.T) {
	small := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 100},
		{Forward: false, SpeedHz: 500, Steps: 10},
	}
	if got := patternBatchRepetitions(small, 40); got != 12 {
		t.Fatalf("expected 12 batch reps for 2-segment pattern, got %d", got)
	}
	if got := patternBatchRepetitions(small, 20); got != 10 {
		t.Fatalf("expected 10 batch reps with ops limit 20, got %d", got)
	}

	sixSeg := make([]StepperRoutineSegment, 6)
	for i := range sixSeg {
		sixSeg[i] = StepperRoutineSegment{Forward: i%2 == 0, SpeedHz: 500, Steps: 100}
	}
	if got := patternBatchRepetitions(sixSeg, 40); got != 6 {
		t.Fatalf("expected 6 batch reps for 6-segment pattern (40/6), got %d", got)
	}
	if got := patternBatchRepetitions(sixSeg, 20); got != 3 {
		t.Fatalf("expected 3 batch reps for 6-segment pattern with 2 routines, got %d", got)
	}
}

func TestRoutineBatchQueueOpsWithinLimit(t *testing.T) {
	sixSeg := func() []StepperRoutineSegment {
		segs := make([]StepperRoutineSegment, 6)
		for i := range segs {
			segs[i] = StepperRoutineSegment{Forward: true, SpeedHz: 500, Steps: 100}
		}
		return segs
	}
	for _, routines := range []int{1, 2} {
		segments := sixSeg()
		limit := routineQueueOpsLimitFor(routines)
		reps := patternBatchRepetitions(segments, limit)
		ops := routineBatchQueueOps(segments, reps)
		if ops > limit {
			t.Fatalf("%d routines: batch queue ops %d exceeds limit %d", routines, ops, limit)
		}
		refill := patternRefillRepetitions(segments, limit)
		refillOps := routineBatchQueueOps(segments, refill)
		if refillOps > limit {
			t.Fatalf("%d routines: refill queue ops %d exceeds limit %d", routines, refillOps, limit)
		}
	}
}

func TestRoutineRefillWait(t *testing.T) {
	segments := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 100},
	}
	wait := routineRefillWait(segments, 1)
	if wait < 60*time.Millisecond || wait > 80*time.Millisecond {
		t.Fatalf("expected ~70ms refill wait for 100 steps @ 500Hz, got %v", wait)
	}
	wait2 := routineRefillWait(segments, 2)
	if wait2 <= wait {
		t.Fatalf("expected longer refill wait with 2 routines, got %v vs %v", wait2, wait)
	}
}

func TestRepeatPatternSegments(t *testing.T) {
	base := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 10},
		{Forward: false, SpeedHz: 500, Steps: 5},
	}
	repeated := repeatPatternSegments(base, 3)
	if len(repeated) != 6 {
		t.Fatalf("expected 6 segments for 3 reps, got %d", len(repeated))
	}
}

func TestPatternTicks(t *testing.T) {
	segments := []StepperRoutineSegment{
		{Forward: true, SpeedHz: 500, Steps: 100},
		{Forward: false, SpeedHz: 1000, Steps: 10},
	}
	// 16 MHz: 32000 ticks per step at 500 Hz, 16000 at 1 kHz.
	assert.Equal(t, uint64(100*32000+10*16000), patternTicks(segments, 16e6))
	assert.Equal(t, 2*patternTicks(segments, 16e6), patternTicks(repeatPatternSegments(segments, 2), 16e6))
}

func TestClockAhead(t *testing.T) {
	assert.Equal(t, int64(500), clockAhead(1000, 1500))
	assert.Equal(t, int64(-500), clockAhead(1500, 1000))
	// Across the 32-bit wrap: the end is just past zero, now is just before it.
	assert.Equal(t, int64(0x20), clockAhead(0xfffffff0, 0x10))
	assert.Equal(t, int64(-0x20), clockAhead(0x10, 0xfffffff0))
}

func TestRoutineStates(t *testing.T) {
	s := &Session{}
	current := &stepperRoutineKeeper{stop: make(chan struct{})}
	replaced := &stepperRoutineKeeper{stop: make(chan struct{})}
	s.routineKeepers = map[int]*stepperRoutineKeeper{2: current, 5: current}

	s.failRoutine(5, current, fmt.Errorf("refill failed"))
	s.failRoutine(2, replaced, fmt.Errorf("stale loop")) // not the current routine: ignored
	delete(s.routineKeepers, 5)                           // the failed loop exits

	assert.Equal(t, []RoutineState{
		{OID: 2, Running: true},
		{OID: 5, Error: "refill failed"},
	}, s.RoutineStates())

	s.stopStepperRoutineLocked(5) // a stop or new routine clears the failure
	assert.Equal(t, []RoutineState{{OID: 2, Running: true}}, s.RoutineStates())
}

func TestValidateRoutineSegments(t *testing.T) {
	if err := validateRoutineSegments(nil); err == nil {
		t.Fatal("expected error for empty segments")
	}
	if err := validateRoutineSegments([]StepperRoutineSegment{{Steps: 10, SpeedHz: 500}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
