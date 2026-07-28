package mcu

import (
	"time"
)

// telemetrySub receives reading snapshots from the session broadcast loop.
type telemetrySub struct {
	ch chan map[string]Reading
}

// Session telemetry broadcasts readings to WebSocket clients.
func (s *Session) SubscribeTelemetry() chan map[string]Reading {
	s.telemMu.Lock()
	defer s.telemMu.Unlock()
	if s.telemSubs == nil {
		s.telemSubs = make(map[chan map[string]Reading]struct{})
	}
	ch := make(chan map[string]Reading, 8)
	s.telemSubs[ch] = struct{}{}
	if !s.telemRunning {
		s.telemRunning = true
		go s.telemetryLoop()
	}
	return ch
}

func (s *Session) UnsubscribeTelemetry(ch chan map[string]Reading) {
	s.telemMu.Lock()
	defer s.telemMu.Unlock()
	if _, ok := s.telemSubs[ch]; ok {
		delete(s.telemSubs, ch)
		close(ch)
	}
}

func (s *Session) telemetryLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		<-ticker.C
		s.telemMu.Lock()
		if len(s.telemSubs) == 0 {
			s.telemRunning = false
			s.telemMu.Unlock()
			return
		}
		subs := make([]chan map[string]Reading, 0, len(s.telemSubs))
		for ch := range s.telemSubs {
			subs = append(subs, ch)
		}
		s.telemMu.Unlock()

		s.RefreshStepperReadings()
		readings := s.Poll(150 * time.Millisecond)
		for _, ch := range subs {
			select {
			case ch <- readings:
			default:
			}
		}
	}
}

// StopTelemetry ends all subscribers (called on session close).
func (s *Session) StopTelemetry() {
	s.telemMu.Lock()
	defer s.telemMu.Unlock()
	for ch := range s.telemSubs {
		close(ch)
	}
	s.telemSubs = nil
	s.telemRunning = false
}
