package sorter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupplyVolts(t *testing.T) {
	s := DefaultMachineSettings().Supply
	// 12 V through 10 kΩ / 4.7 kΩ is 3.837 V at the pin: 0.767 of the Mega's 5 V.
	fraction := 12 * 4700.0 / 14700 / 5
	assert.InDelta(t, 12, s.Volts(fraction), 1e-9)
	s.Calibration = 1.02
	assert.InDelta(t, 12.24, s.Volts(fraction), 1e-9)
	assert.InDelta(t, 2.5, SupplySettings{}.Volts(0.5), 1e-9, "no divider, 5 V reference")
}

func TestMachineRefusesToMoveWithoutMotorPower(t *testing.T) {
	m, hw := startMachine(t)
	hw.SetSupply(4.2)
	err := do(t, m, CommandHome)
	assert.ErrorIs(t, err, ErrSupplyLow)
	assert.Contains(t, err.Error(), "4.2 V, needs 10.5 V")
	assert.Equal(t, StateStandby, m.Snapshot().State)

	hw.SetSupply(12.1)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	hw.SetSupply(3)
	assert.ErrorIs(t, do(t, m, CommandStart), ErrSupplyLow)
}

func TestMachineStopsWhenMotorPowerGoes(t *testing.T) {
	m, hw := startMachine(t)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	require.NoError(t, do(t, m, CommandStart))
	hw.SetSupply(0.4) // the supply was switched off
	require.Eventually(t, func() bool { return m.Snapshot().State == StateError }, 3*time.Second, 10*time.Millisecond)
	snap := m.Snapshot()
	assert.Contains(t, snap.Error, "supply is low")
	assert.True(t, snap.Supply.Low)
	assert.False(t, snap.Lights.On, "stopping turned the light off")
}
