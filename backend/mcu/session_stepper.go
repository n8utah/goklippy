package mcu

import (
	"fmt"
	"log/slog"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

func (s *Session) stepperDriverEnableCommands(stepperOID int) []string {
	section := ""
	for _, obj := range s.Objects {
		if obj.OID == stepperOID && obj.Kind == cfgtranslate.KindStepper {
			section = obj.Section
			break
		}
	}
	if section == "" {
		return nil
	}
	for _, obj := range s.Objects {
		if obj.Meta["stepper"] != section {
			continue
		}
		val := 1
		if obj.Meta["active_low"] == "true" {
			val = 0
		}
		return []string{fmt.Sprintf("update_digital_out oid=%d value=%d", obj.OID, val)}
	}
	return nil
}

// ensureMCUReady clears an earlier shutdown before new motion is sent. A shutdown has
// already killed every move on the MCU, so anything still marked running is ended here.
func (s *Session) ensureMCUReady() error {
	s.recoverMu.Lock()
	defer s.recoverMu.Unlock()

	status, err := s.Conn.GetConfig()
	if err != nil {
		return err
	}
	if !status.IsShutdown {
		return nil
	}
	s.abortAllMotion(fmt.Errorf("mcu shut down: %s", s.Conn.ShutdownReason()))
	if err := s.recoverMCUShutdown(); err != nil {
		return fmt.Errorf("mcu shutdown could not be cleared, use Reset: %w", err)
	}
	slog.Info("mcu recovered from shutdown", "port", s.Port)
	return nil
}

// abortAllMotion forgets all running jogs and routines without talking to the MCU.
func (s *Session) abortAllMotion(reason error) {
	s.stopAllJogKeepers()
	s.mu.Lock()
	s.stepperJogging = make(map[int]bool)
	s.mu.Unlock()

	s.routineKeepersMu.Lock()
	defer s.routineKeepersMu.Unlock()
	if s.routineErrors == nil {
		s.routineErrors = make(map[int]string)
	}
	for oid, state := range s.routineKeepers {
		if !state.stopped() {
			close(state.stop)
		}
		s.routineErrors[oid] = reason.Error()
		delete(s.routineKeepers, oid)
	}
}

// failIfShutdown turns a shutdown caused by commands just sent into an error, so the
// caller doesn't report a move the MCU refused.
func (s *Session) failIfShutdown() error {
	shutdown, err := s.mcuIsShutdown()
	if err != nil {
		return err
	}
	if shutdown {
		return fmt.Errorf("mcu shut down: %s", s.Conn.ShutdownReason())
	}
	return nil
}

func (s *Session) mcuIsShutdown() (bool, error) {
	status, err := s.Conn.GetConfig()
	if err != nil {
		return false, err
	}
	return status.IsShutdown, nil
}

func (s *Session) recoverMCUShutdown() error {
	if err := s.Conn.ClearShutdown(); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	for _, line := range s.InitCommands {
		if err := s.Conn.SendCommand(line); err != nil {
			return fmt.Errorf("re-init %q: %w", line, err)
		}
	}
	status, err := s.Conn.GetConfig()
	if err != nil {
		return err
	}
	if status.IsShutdown {
		return fmt.Errorf("mcu still shutdown after clear and re-init")
	}
	return nil
}
