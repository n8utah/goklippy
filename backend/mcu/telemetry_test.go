package mcu

import (
	"testing"
	"time"
)

func TestTelemetryUnsubscribeDuringBroadcast(t *testing.T) {
	s := &Session{
		readings:       make(map[int]Reading),
		stepperJogging: make(map[int]bool),
		stopTrsyncOIDs: make(map[int]int),
	}

	ch := s.SubscribeTelemetry()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()

	for i := 0; i < 50; i++ {
		sub := s.SubscribeTelemetry()
		go func(c chan map[string]Reading) {
			time.Sleep(time.Duration(i%5) * time.Millisecond)
			s.UnsubscribeTelemetry(c)
		}(sub)
	}

	time.Sleep(20 * time.Millisecond)
	s.UnsubscribeTelemetry(ch)
	s.StopTelemetry()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("telemetry subscriber did not exit")
	}
}
