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

func (s *Session) checkMCUShutdown(context string) error {
	status, err := s.Conn.GetConfig()
	if err != nil {
		return err
	}
	if !status.IsShutdown {
		return nil
	}
	slog.Warn("mcu shutdown detected",
		"port", s.Port,
		"context", context,
		"crc", status.CRC,
		"move_count", status.MoveCount,
	)
	if err := s.recoverMCUShutdown(); err != nil {
		return fmt.Errorf("mcu shutdown — disconnect and reconnect: %w", err)
	}
	slog.Info("mcu recovered from shutdown", "port", s.Port, "context", context)
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
