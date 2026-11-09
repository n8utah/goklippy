package mcu

import (
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

const (
	motionSamplePeriod  = 50 * time.Millisecond
	motionPublishEvery  = 4 // samples between snapshots (5 per second)
	motionSpeedInterval = 5 // intervals (250 ms) behind the speed reading, so reversals show
	motionHistorySize   = motionSpeedInterval + 1
)

// StepperMotion is a stepper's motion as measured from the MCU's step position. It
// reflects the steps the MCU issued; the steppers are open-loop, so a skipped step on
// the shaft can't be seen here.
type StepperMotion struct {
	OID         int     `json:"oid"`
	Position    int64   `json:"position"`
	StepsPerSec float64 `json:"steps_per_sec"` // signed: negative is reverse
	Direction   int     `json:"direction"`     // 1 forward, -1 reverse, 0 stopped
}

// MotionSnapshot is one published set of readings for every stepper.
type MotionSnapshot struct {
	At             time.Time       `json:"at"`
	Steppers       []StepperMotion `json:"steppers"`
	ShutdownReason string          `json:"shutdown_reason,omitempty"`
}

type motionSample struct {
	at  time.Time
	pos int64
}

func computeMotion(oid int, samples []motionSample) StepperMotion {
	out := StepperMotion{OID: oid}
	if len(samples) == 0 {
		return out
	}
	last := samples[len(samples)-1]
	out.Position = last.pos
	if len(samples) < 2 {
		return out
	}

	first := samples[max(0, len(samples)-1-motionSpeedInterval)]
	if dt := last.at.Sub(first.at).Seconds(); dt > 0 {
		out.StepsPerSec = float64(last.pos-first.pos) / dt
	}
	switch {
	case out.StepsPerSec > 0:
		out.Direction = 1
	case out.StepsPerSec < 0:
		out.Direction = -1
	}
	return out
}

func (s *Session) stepperOIDs() []int {
	var oids []int
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, obj := range s.Objects {
		if obj.Kind == cfgtranslate.KindStepper {
			oids = append(oids, obj.OID)
		}
	}
	return oids
}

// SubscribeMotion delivers motion snapshots until unsubscribed. Sampling runs only
// while someone is listening.
func (s *Session) SubscribeMotion() chan MotionSnapshot {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	if s.motionSubs == nil {
		s.motionSubs = make(map[chan MotionSnapshot]struct{})
	}
	ch := make(chan MotionSnapshot, 4)
	s.motionSubs[ch] = struct{}{}
	if s.motionStop == nil {
		s.motionStop = make(chan struct{})
		s.motionDone = make(chan struct{})
		go s.motionLoop(s.motionStop, s.motionDone)
	}
	return ch
}

// UnsubscribeMotion removes a subscriber, ending sampling after the last one.
func (s *Session) UnsubscribeMotion(ch chan MotionSnapshot) {
	s.motionMu.Lock()
	if _, ok := s.motionSubs[ch]; ok {
		delete(s.motionSubs, ch)
		close(ch)
	}
	idle := len(s.motionSubs) == 0
	s.motionMu.Unlock()
	if idle {
		s.stopMotionLoop()
	}
}

// stopMotionLoop ends sampling and waits for it, so the connection can close safely.
func (s *Session) stopMotionLoop() {
	s.motionMu.Lock()
	stop, done := s.motionStop, s.motionDone
	s.motionStop, s.motionDone = nil, nil
	s.motionMu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

// stopMotion ends sampling and closes every subscriber (session close).
func (s *Session) stopMotion() {
	s.stopMotionLoop()
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	for ch := range s.motionSubs {
		close(ch)
	}
	s.motionSubs = nil
}

func (s *Session) motionLoop(stop, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(motionSamplePeriod)
	defer ticker.Stop()

	oids := s.stepperOIDs()
	history := make(map[int][]motionSample, len(oids))
	for tick := 1; ; tick++ {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if s.resetting.Load() {
			clear(history)
			continue
		}

		for _, oid := range oids {
			pos, err := s.StepperPosition(oid)
			if err != nil {
				continue
			}
			samples := append(history[oid], motionSample{at: time.Now(), pos: pos})
			if len(samples) > motionHistorySize {
				samples = samples[len(samples)-motionHistorySize:]
			}
			history[oid] = samples
		}

		if tick%motionPublishEvery != 0 {
			continue
		}
		snapshot := MotionSnapshot{
			At:             time.Now().UTC(),
			Steppers:       make([]StepperMotion, 0, len(oids)),
			ShutdownReason: s.Conn.ShutdownReason(),
		}
		for _, oid := range oids {
			snapshot.Steppers = append(snapshot.Steppers, computeMotion(oid, history[oid]))
		}
		s.publishMotion(snapshot)
	}
}

func (s *Session) publishMotion(snapshot MotionSnapshot) {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	for ch := range s.motionSubs {
		select {
		case ch <- snapshot:
		default: // a slow client skips a frame rather than delaying sampling
		}
	}
}
