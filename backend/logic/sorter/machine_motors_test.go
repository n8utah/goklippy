package sorter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMachineLetsIdleMotorsGo(t *testing.T) {
	hw := NewSimHardware(DefaultMachineSettings)
	m := NewMachine(hw, DefaultMachineSettings, 0)
	now := time.Now()

	m.state = StateRunning
	m.releaseIdleMotors(now)
	assert.Zero(t, hw.Releases(), "sorting: held")

	m.state = StatePaused
	m.releaseIdleMotors(now.Add(time.Second))
	assert.Zero(t, hw.Releases(), "just paused: a move may still be finishing")
	m.releaseIdleMotors(now.Add(motorsIdleFor + time.Millisecond))
	assert.Equal(t, 1, hw.Releases(), "paused a while: let go")
	m.releaseIdleMotors(now.Add(time.Minute))
	assert.Equal(t, 1, hw.Releases(), "once, until something moves them again")

	m.state = StateRunning
	m.releaseIdleMotors(now.Add(2 * time.Minute))
	m.incident = &IncidentHold{}
	m.releaseIdleMotors(now.Add(2*time.Minute + motorsIdleFor + time.Millisecond))
	assert.Equal(t, 2, hw.Releases(), "waiting on the operator counts as idle")
}
