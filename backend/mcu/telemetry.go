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
	defer func() {
		s.telemMu.Lock()
		s.telemRunning = false
		s.telemMu.Unlock()
	}()
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

		readings := s.Poll(50 * time.Millisecond)

		s.telemMu.Lock()
		for _, ch := range subs {
			if _, active := s.telemSubs[ch]; !active {
				continue
			}
			select {
			case ch <- readings:
			default:
			}
		}
		if len(s.telemSubs) == 0 {
			s.telemRunning = false
		}
		s.telemMu.Unlock()
	}
}

// StopTelemetry ends all subscribers (called on session close).
func (s *Session) StopTelemetry() {
	s.telemMu.Lock()
	subs := s.telemSubs
	s.telemSubs = nil
	s.telemRunning = false
	s.telemMu.Unlock()

	for ch := range subs {
		close(ch)
	}
}
