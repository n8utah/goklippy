package mcu

import (
	"fmt"
	"log/slog"
	"time"
)

// scheduleClock returns an MCU clock tick marginSec in the future using the
// host-estimated clock baseline synced at connect.
func (s *Session) scheduleClock(marginSec float64) uint64 {
	return (s.EstimatedClock() + uint64(s.ClockFreq*marginSec)) & 0xffffffff
}

// sendAtMotionClock sends commands built around a start time marginSec from now, and
// returns that start time. The clock read and the send are one step on the wire.
func (s *Session) sendAtMotionClock(marginSec float64, build func(startClock uint64) []string) (uint64, error) {
	margin := uint32(s.ClockFreq * marginSec)
	startClock, err := s.Conn.SendAtClock(margin, func(start uint32) []string {
		return build(uint64(start))
	})
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.ClockBase = uint64(startClock - margin)
	s.ClockBaseAt = time.Now()
	s.mu.Unlock()
	return uint64(startClock), nil
}

// motionClock returns a future MCU clock tick for scheduling moves. It prefers
// a live get_clock query (accurate for steppers) and falls back to the host
// estimate when the serial port is busy.
func (s *Session) motionClock(marginSec float64) (uint64, error) {
	mcuClock, err := s.Conn.MCUClock()
	if err != nil {
		if s.ClockBaseAt.IsZero() {
			return 0, fmt.Errorf("get mcu clock: %w", err)
		}
		slog.Warn("get_clock failed, using estimated mcu clock",
			"port", s.Port,
			"error", err,
		)
		return s.scheduleClock(marginSec), nil
	}
	s.mu.Lock()
	s.ClockBase = uint64(mcuClock)
	s.ClockBaseAt = time.Now()
	s.mu.Unlock()
	return (uint64(mcuClock) + uint64(s.ClockFreq*marginSec)) & 0xffffffff, nil
}
