package mcu

import (
	"fmt"
)

const (
	servoPeriodSec   = 0.020
	servoClockMargin = 0.200 // match Klipper pwm_tool init scheduling
)

func servoOnTicks(clockFreq float64, angle float64) int {
	if angle < 0 {
		angle = 0
	}
	if angle > 180 {
		angle = 180
	}
	widthSec := 0.001 + (angle/180.0)*0.001
	cycleTicks := int(clockFreq * servoPeriodSec)
	onTicks := int(float64(cycleTicks) * (widthSec / servoPeriodSec))
	if onTicks < 1 {
		onTicks = 1
	}
	if onTicks > cycleTicks {
		onTicks = cycleTicks
	}
	return onTicks
}

func (s *Session) queueServoPulse(oid int, angle float64) error {
	onTicks := servoOnTicks(s.ClockFreq, angle)
	clock, err := s.motionClock(servoClockMargin)
	if err != nil {
		return err
	}
	return s.Conn.SendCommand(fmt.Sprintf(
		"queue_digital_out oid=%d clock=%d on_ticks=%d",
		oid, clock, onTicks,
	))
}
