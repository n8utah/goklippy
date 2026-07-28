package mcu

import (
	"fmt"
	"time"
)

const (
	maxJogSpeedHz     = 50000.0
	maxQueueStepCount = 65535
)

type stepperJogKeeper struct {
	forward bool
	speedHz float64
	stop    chan struct{}
}

func clampJogSpeed(speedHz float64) float64 {
	if speedHz < 0.1 {
		return 0.1
	}
	if speedHz > maxJogSpeedHz {
		return maxJogSpeedHz
	}
	return speedHz
}

func jogKeeperWait(speedHz float64) time.Duration {
	speedHz = clampJogSpeed(speedHz)
	batchSec := float64(maxQueueStepCount) / speedHz
	waitSec := batchSec * 0.6
	if waitSec < 0.25 {
		waitSec = 0.25
	}
	if waitSec > 20 {
		waitSec = 20
	}
	return time.Duration(waitSec * float64(time.Second))
}

func (s *Session) startJogKeeper(oid int, forward bool, speedHz float64) {
	s.jogMu.Lock()
	if old, ok := s.jogKeepers[oid]; ok {
		close(old.stop)
	}
	state := &stepperJogKeeper{
		forward: forward,
		speedHz: clampJogSpeed(speedHz),
		stop:    make(chan struct{}),
	}
	if s.jogKeepers == nil {
		s.jogKeepers = make(map[int]*stepperJogKeeper)
	}
	s.jogKeepers[oid] = state
	s.jogMu.Unlock()

	go s.jogKeeperLoop(oid, state)
}

func (s *Session) updateJogKeeper(oid int, forward bool, speedHz float64) {
	s.jogMu.Lock()
	defer s.jogMu.Unlock()
	if state, ok := s.jogKeepers[oid]; ok {
		state.forward = forward
		state.speedHz = clampJogSpeed(speedHz)
	}
}

func (s *Session) stopJogKeeper(oid int) {
	s.jogMu.Lock()
	defer s.jogMu.Unlock()
	if state, ok := s.jogKeepers[oid]; ok {
		close(state.stop)
		delete(s.jogKeepers, oid)
	}
}

func (s *Session) stopAllJogKeepers() {
	s.jogMu.Lock()
	defer s.jogMu.Unlock()
	for oid, state := range s.jogKeepers {
		close(state.stop)
		delete(s.jogKeepers, oid)
	}
}

func (s *Session) jogKeeperLoop(oid int, state *stepperJogKeeper) {
	for {
		s.jogMu.Lock()
		speedHz := state.speedHz
		stop := state.stop
		s.jogMu.Unlock()

		timer := time.NewTimer(jogKeeperWait(speedHz))
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}

		if !s.isStepperJogging(oid) {
			return
		}

		s.jogMu.Lock()
		forward := state.forward
		speedHz = state.speedHz
		stop = state.stop
		s.jogMu.Unlock()

		select {
		case <-stop:
			return
		default:
		}

		if err := s.stepperJogAppend(oid, forward, speedHz); err != nil {
			s.setStepperJogging(oid, false)
			s.stopJogKeeper(oid)
			return
		}
	}
}

func (s *Session) stepperJogAppend(oid int, forward bool, speedHz float64) error {
	speedHz = clampJogSpeed(speedHz)
	dir := 0
	if forward {
		dir = 1
	}
	interval := int(s.ClockFreq / speedHz)
	if interval < 1 {
		interval = 1
	}
	cmds := []string{
		fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir),
		fmt.Sprintf("queue_step oid=%d interval=%d count=%d add=0", oid, interval, maxQueueStepCount),
	}
	return s.Conn.SendCommands(cmds)
}

func queueStepCommands(oid, interval, steps int) []string {
	if steps <= 0 {
		return []string{
			fmt.Sprintf("queue_step oid=%d interval=%d count=%d add=0", oid, interval, maxQueueStepCount),
		}
	}
	cmds := make([]string, 0, 1+steps/maxQueueStepCount)
	remaining := steps
	for remaining > 0 {
		count := remaining
		if count > maxQueueStepCount {
			count = maxQueueStepCount
		}
		cmds = append(cmds, fmt.Sprintf("queue_step oid=%d interval=%d count=%d add=0", oid, interval, count))
		remaining -= count
	}
	return cmds
}
