package mcu

import (
	"fmt"
	"log/slog"
	"time"
)

// Reset restarts the board by reopening its serial port and reconfigures it, keeping
// the session. All motion is abandoned; the reset itself stops the motors.
func (s *Session) Reset() error {
	s.recoverMu.Lock()
	defer s.recoverMu.Unlock()

	s.resetting.Store(true)
	defer s.resetting.Store(false)

	start := time.Now()
	s.abortAllMotion(fmt.Errorf("board was reset"))

	if err := s.Conn.Reopen(s.Baud); err != nil {
		return fmt.Errorf("reset board: %w", err)
	}
	if err := s.Conn.ConfigureWithInit(s.ConfigLines, s.InitCommands); err != nil {
		return fmt.Errorf("reconfigure board: %w", err)
	}
	if err := s.SyncClock(); err != nil {
		return fmt.Errorf("sync clock: %w", err)
	}

	s.forgetOutputLevels()

	s.pcaMu.Lock()
	s.pcaReady = nil
	s.pcaMu.Unlock()
	s.InitPCA9685Boards()

	slog.Info("mcu board reset", "port", s.Port, "duration_ms", time.Since(start).Milliseconds())
	return nil
}
