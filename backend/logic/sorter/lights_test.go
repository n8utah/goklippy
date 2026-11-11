package sorter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineSwitchesLights(t *testing.T) {
	m, hw := startMachine(t)
	ctx := context.Background()
	assert.False(t, m.Snapshot().Lights.On)

	require.NoError(t, m.SetLights(ctx, true))
	assert.True(t, m.Snapshot().Lights.On)
	assert.True(t, hw.LightsStatus().On)

	require.NoError(t, m.SetLights(ctx, false))
	assert.False(t, m.Snapshot().Lights.On)

	hw.SetConnected(false)
	assert.ErrorIs(t, m.SetLights(ctx, true), ErrNoBoard)
	assert.NotEmpty(t, m.Snapshot().Lights.Message)
}

func TestMachineLightsOnWhenSorting(t *testing.T) {
	m, _ := startMachine(t)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	require.NoError(t, do(t, m, CommandStart))
	assert.True(t, m.Snapshot().Lights.On, "starting turns the light on")

	require.NoError(t, do(t, m, CommandPause))
	require.NoError(t, m.SetLights(context.Background(), false))
	assert.False(t, m.Snapshot().Lights.On, "the light can be switched off while paused")
	require.NoError(t, do(t, m, CommandResume))
	assert.True(t, m.Snapshot().Lights.On, "resuming turns it back on")

	require.NoError(t, do(t, m, CommandStop))
	assert.False(t, m.Snapshot().Lights.On, "stopping always turns it off")
}

func TestMachineLightsOffWhenAnErrorStopsIt(t *testing.T) {
	m, hw := startMachine(t)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	require.NoError(t, do(t, m, CommandStart))
	require.True(t, hw.LightsStatus().On)

	require.NoError(t, m.send(context.Background(), request{do: func() error {
		m.stopForError("test error")
		return nil
	}}))
	assert.False(t, hw.LightsStatus().On)
	assert.False(t, m.Snapshot().Lights.On)
}

func TestMachineLightsOffWhenTheBackendExits(t *testing.T) {
	settings := func() MachineSettings { return DefaultMachineSettings() }
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	m := NewMachine(hw, settings, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)

	require.NoError(t, m.SetLights(context.Background(), true))
	cancel()
	<-m.Done()
	assert.False(t, hw.LightsStatus().On)
}
