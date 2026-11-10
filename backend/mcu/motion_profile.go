package mcu

import (
	"fmt"
	"math"
)

// Accelerated moves. The MCU only knows queue_step: "first step interval ticks after
// the last one, then count-1 more, each interval growing by add". A trapezoid speed
// profile is turned into exact step times, and those are packed into as few queue_step
// chunks as fit within a timing tolerance — the same idea as Klipper's stepcompress.

const (
	// defaultStepToleranceSec is how far a compressed step may land from its exact time.
	// Klipper's default max_stepper_error is 25 µs.
	defaultStepToleranceSec = 25e-6
	// maxMoveQueueOps caps how many queue_step commands one move may queue on the MCU.
	// They go out maxQueueOpsPerBatch at a time (see StepperMove), so the cap is about
	// the MCU's shared move pool, not the serial line.
	maxMoveQueueOps = 256
	minAddValue     = math.MinInt16
	maxAddValue     = math.MaxInt16
)

// TrapezoidMove is a point-to-point move that ramps up to MaxHz and back down.
type TrapezoidMove struct {
	Steps   int
	StartHz float64 // speed at the first and last step; 0 starts from rest
	MaxHz   float64
	AccelHz float64 // steps per second²; 0 means no ramp (constant MaxHz)
}

// queueStep is one queue_step command.
type queueStep struct {
	Interval int
	Count    int
	Add      int
}

// stepTimes returns the time, in seconds from the move start, of each step 1..Steps.
func (m TrapezoidMove) stepTimes() []float64 {
	n := m.Steps
	times := make([]float64, n)
	if n <= 0 {
		return times
	}
	v := clampJogSpeed(m.MaxHz)
	if m.AccelHz <= 0 {
		for k := 1; k <= n; k++ {
			times[k-1] = float64(k) / v
		}
		return times
	}
	a := m.AccelHz
	v0 := math.Max(0, math.Min(m.StartHz, v))
	// accelTime is the time to cover x steps while accelerating from v0.
	accelTime := func(x float64) float64 {
		return (math.Sqrt(v0*v0+2*a*x) - v0) / a
	}
	rampSteps := (v*v - v0*v0) / (2 * a)
	var total float64
	if 2*rampSteps >= float64(n) {
		// Never reaches MaxHz: accelerate to the middle, then decelerate.
		rampSteps = float64(n) / 2
		total = 2 * accelTime(rampSteps)
	} else {
		total = 2*accelTime(rampSteps) + (float64(n)-2*rampSteps)/v
	}
	for k := 1; k <= n; k++ {
		x := float64(k)
		switch {
		case x <= rampSteps:
			times[k-1] = accelTime(x)
		case x >= float64(n)-rampSteps:
			times[k-1] = total - accelTime(float64(n)-x)
		default:
			times[k-1] = accelTime(rampSteps) + (x-rampSteps)/v
		}
	}
	return times
}

// Duration is how long the move takes from its start to its last step.
func (m TrapezoidMove) Duration() float64 {
	times := m.stepTimes()
	if len(times) == 0 {
		return 0
	}
	return times[len(times)-1]
}

// compressSteps packs absolute step clocks (ticks after the move start) into queue_step
// chunks. Every step lands within tol ticks of its exact time. Each chunk is fitted
// from where the previous chunk's last step really landed, so error can't build up.
func compressSteps(ticks []float64, tol float64) []queueStep {
	var out []queueStep
	last := 0.0 // clock of the last step actually scheduled
	for i := 0; i < len(ticks); {
		// A single step always fits, to the tick.
		best := queueStep{Interval: max(1, int(math.Round(ticks[i]-last))), Count: 1}
		// Grow the chunk while it still fits: double, then binary-search the edge.
		lo, hi := 1, 1
		for hi < len(ticks)-i && hi < maxQueueStepCount {
			next := min(hi*2, len(ticks)-i, maxQueueStepCount)
			if _, ok := tryChunk(ticks[i:], last, next, tol); !ok {
				hi = next
				break
			}
			lo, hi = next, next
		}
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			if _, ok := tryChunk(ticks[i:], last, mid, tol); ok {
				lo = mid
			} else {
				hi = mid
			}
		}
		if chunk, ok := tryChunk(ticks[i:], last, lo, tol); ok {
			best = chunk
		}
		out = append(out, best)
		last = chunkEnd(last, best)
		i += best.Count
	}
	return out
}

// tryChunk fits count steps with one interval/add pair and reports whether every step
// lands within tol. Step j (0-based) lands at last + (j+1)*interval + add*j*(j+1)/2,
// which is linear in interval and add, so they are fitted by least squares and the
// integer pairs around the fit are checked.
func tryChunk(ticks []float64, last float64, count int, tol float64) (queueStep, bool) {
	if count <= 0 || count > len(ticks) {
		return queueStep{}, false
	}
	if count == 1 {
		interval := int(math.Round(ticks[0] - last))
		ok := interval >= 1 && math.Abs(last+float64(interval)-ticks[0]) <= tol
		return queueStep{Interval: interval, Count: 1}, ok
	}
	var sxx, sxy, syy, sxz, syz float64
	for j := 0; j < count; j++ {
		x := float64(j + 1)
		y := float64(j) * float64(j+1) / 2
		z := ticks[j] - last
		sxx += x * x
		sxy += x * y
		syy += y * y
		sxz += x * z
		syz += y * z
	}
	det := sxx*syy - sxy*sxy
	if det == 0 {
		return queueStep{}, false
	}
	fitAdd := (sxx*syz - sxy*sxz) / det
	// add has to be whole, so for each whole add either side of the fit, the interval
	// is re-fitted with that add held fixed.
	baseA := int(math.Floor(fitAdd))
	for da := 0; da <= 1; da++ {
		add := baseA + da
		fitInterval := (sxz - float64(add)*sxy) / sxx
		baseI := int(math.Floor(fitInterval))
		for di := 0; di <= 1; di++ {
			q := queueStep{Interval: baseI + di, Count: count, Add: add}
			if chunkFits(ticks, last, q, tol) {
				return q, true
			}
		}
	}
	return queueStep{}, false
}

func chunkFits(ticks []float64, last float64, q queueStep, tol float64) bool {
	if q.Interval < 1 || q.Add < minAddValue || q.Add > maxAddValue {
		return false
	}
	step := float64(q.Interval)
	at := last
	for j := 0; j < q.Count; j++ {
		if step < 1 {
			return false
		}
		at += step
		if math.Abs(at-ticks[j]) > tol {
			return false
		}
		step += float64(q.Add)
	}
	return true
}

// chunkEnd is the clock of a chunk's last step, given the clock before it.
func chunkEnd(last float64, q queueStep) float64 {
	c := float64(q.Count)
	return last + c*float64(q.Interval) + float64(q.Add)*(c-1)*c/2
}

// moveQueueSteps compresses a move for an MCU running at clockFreq. If the result needs
// more commands than one batch allows, the tolerance is relaxed until it fits.
func moveQueueSteps(m TrapezoidMove, clockFreq float64) ([]queueStep, error) {
	if m.Steps <= 0 {
		return nil, fmt.Errorf("move needs at least one step")
	}
	if m.MaxHz <= 0 {
		return nil, fmt.Errorf("move speed must be positive")
	}
	times := m.stepTimes()
	ticks := make([]float64, len(times))
	for i, t := range times {
		ticks[i] = t * clockFreq
	}
	tol := defaultStepToleranceSec * clockFreq
	for attempt := 0; attempt < 6; attempt++ {
		steps := compressSteps(ticks, tol)
		if len(steps) <= maxMoveQueueOps {
			return steps, nil
		}
		tol *= 2
	}
	return nil, fmt.Errorf("move of %d steps needs too many step commands; lower the acceleration or speed", m.Steps)
}

// queueStepsTicks is how many ticks the chunks take, start to last step.
func queueStepsTicks(steps []queueStep) uint64 {
	var last float64
	for _, q := range steps {
		last = chunkEnd(last, q)
	}
	return uint64(math.Round(last))
}

func queueStepCommandsFor(oid int, steps []queueStep) []string {
	cmds := make([]string, 0, len(steps))
	for _, q := range steps {
		cmds = append(cmds, fmt.Sprintf("queue_step oid=%d interval=%d count=%d add=%d", oid, q.Interval, q.Count, q.Add))
	}
	return cmds
}
