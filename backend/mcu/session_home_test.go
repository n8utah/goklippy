package mcu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHomeCommands(t *testing.T) {
	s := &Session{ClockFreq: 16e6}
	cmds := s.homeCommands(HomeMove{
		StepperOID: 4, EndstopOID: 9, Forward: false, SpeedHz: 1000, MaxSteps: 5000,
	}, 12, 1_000_000)

	assert.Equal(t, []string{
		"trsync_trigger oid=12 reason=1",
		"trsync_start oid=12 report_clock=0 report_ticks=0 expire_reason=0",
		"stepper_stop_on_trigger oid=4 trsync_oid=12",
		"reset_step_clock oid=4 clock=1000000",
		"endstop_home oid=9 clock=1000000 sample_ticks=240 sample_count=4 rest_ticks=8000 pin_value=1 trsync_oid=12 trigger_reason=1",
		"set_next_step_dir oid=4 dir=0",
		"queue_step oid=4 interval=16000 count=5000 add=0",
	}, cmds)
}

func TestHomeCommandsInvertedSwitch(t *testing.T) {
	s := &Session{ClockFreq: 16e6}
	cmds := s.homeCommands(HomeMove{
		StepperOID: 4, EndstopOID: 9, Forward: true, SpeedHz: 1000, MaxSteps: 10, Invert: true,
	}, 12, 0)
	assert.Contains(t, cmds, "endstop_home oid=9 clock=0 sample_ticks=240 sample_count=4 rest_ticks=8000 pin_value=0 trsync_oid=12 trigger_reason=1")
	assert.Contains(t, cmds, "set_next_step_dir oid=4 dir=1")
}
