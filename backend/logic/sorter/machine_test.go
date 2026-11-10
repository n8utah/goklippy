package sorter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMachine(t *testing.T) (*Machine, *SimHardware) {
	t.Helper()
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.DoorSettleMs = 0
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	m := NewMachine(hw, settings, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go m.Run(ctx)
	return m, hw
}

func waitForState(t *testing.T, m *Machine, want State) {
	t.Helper()
	require.Eventually(t, func() bool { return m.Snapshot().State == want }, time.Second, 5*time.Millisecond,
		"state is %s, want %s", m.Snapshot().State, want)
}

func do(t *testing.T, m *Machine, cmd Command) error {
	t.Helper()
	_, err := m.Do(context.Background(), cmd)
	return err
}

func TestMachineLifecycle(t *testing.T) {
	m, hw := startMachine(t)
	assert.Equal(t, StateStandby, m.Snapshot().State)

	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	assert.Equal(t, -1, hw.Layer(), "homing closes every layer")
	snap := m.Snapshot()
	require.True(t, snap.Chute.Homed)
	assert.InDelta(t, DefaultMachineSettings().Distributor.FirstBinAngle(), *snap.Chute.Angle, 1e-9, "parked at bin 1")
	require.NotNil(t, snap.LastJob)
	assert.Equal(t, "Home", snap.LastJob.Name)
	assert.Empty(t, snap.LastJob.Error)

	require.NoError(t, do(t, m, CommandStart))
	assert.Equal(t, StateRunning, m.Snapshot().State)
	require.Eventually(t, func() bool { return m.Snapshot().Ticks > 2 }, time.Second, 5*time.Millisecond)

	require.NoError(t, do(t, m, CommandPause))
	assert.Equal(t, StatePaused, m.Snapshot().State)
	ticks := m.Snapshot().Ticks
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, ticks, m.Snapshot().Ticks, "no ticks while paused")

	require.NoError(t, do(t, m, CommandResume))
	assert.Equal(t, StateRunning, m.Snapshot().State)

	require.NoError(t, do(t, m, CommandStop))
	assert.Equal(t, StateReady, m.Snapshot().State)
}

func TestMachineRefusesCommandsOutOfOrder(t *testing.T) {
	m, _ := startMachine(t)
	for _, cmd := range []Command{CommandStart, CommandPause, CommandResume, CommandStop} {
		assert.ErrorIs(t, do(t, m, cmd), ErrInvalidTransition, "%s from standby", cmd)
	}
	assert.Error(t, do(t, m, Command("dance")))
	assert.Equal(t, StateStandby, m.Snapshot().State)
}

func TestMachineHomingFailsWithoutBoard(t *testing.T) {
	m, hw := startMachine(t)
	hw.SetConnected(false)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateError)
	assert.Contains(t, m.Snapshot().Error, "no board connected")

	// Homing again once the board is back recovers.
	hw.SetConnected(true)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	assert.Empty(t, m.Snapshot().Error)
}

func TestMachineErrorsWhenBoardLostWhileRunning(t *testing.T) {
	m, hw := startMachine(t)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	require.NoError(t, do(t, m, CommandStart))

	hw.SetConnected(false)
	require.Eventually(t, func() bool { return m.Snapshot().State == StateError }, 3*time.Second, 10*time.Millisecond)
	assert.Contains(t, m.Snapshot().Error, "Board disconnected")
}

func homedMachine(t *testing.T) (*Machine, *SimHardware) {
	t.Helper()
	m, hw := startMachine(t)
	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	return m, hw
}

func waitForJob(t *testing.T, m *Machine) JobStatus {
	t.Helper()
	require.Eventually(t, func() bool { return m.Snapshot().Job == nil }, 2*time.Second, 5*time.Millisecond)
	require.NotNil(t, m.Snapshot().LastJob)
	return *m.Snapshot().LastJob
}

func TestMachineSendToBin(t *testing.T) {
	m, hw := homedMachine(t)
	ctx := context.Background()
	require.NoError(t, m.SendToBin(ctx, BinAddress{Layer: 2, Bin: 5}))
	last := waitForJob(t, m)
	assert.Empty(t, last.Error)

	want, err := DefaultMachineSettings().Distributor.BinPosition(BinAddress{Layer: 2, Bin: 5})
	require.NoError(t, err)
	assert.InDelta(t, want.Angle, hw.Position(ChannelDistributor), 1e-9)
	assert.Equal(t, 1, hw.Layer(), "layer 2's servo is selected")

	assert.Error(t, m.SendToBin(ctx, BinAddress{Layer: 9, Bin: 1}))
	assert.Error(t, m.SendToBin(ctx, BinAddress{Layer: 1, Bin: 99}))
}

func TestMachineSendToBinNeedsHomedChute(t *testing.T) {
	m, _ := startMachine(t)
	ctx := context.Background()
	assert.ErrorIs(t, m.SendToBin(ctx, BinAddress{Layer: 1, Bin: 1}), ErrChuteNotHomed)
	assert.ErrorIs(t, m.TestLayer(ctx, 1, 0), ErrChuteNotHomed)
	assert.ErrorIs(t, m.AimChute(ctx, 30), ErrChuteNotHomed)
	assert.Nil(t, m.Snapshot().LastJob, "nothing started")

	require.NoError(t, m.ZeroChute(ctx))
	require.NoError(t, m.AimChute(ctx, 30))
	assert.Empty(t, waitForJob(t, m).Error)
}

func TestMachineRejectsBadBins(t *testing.T) {
	m, _ := homedMachine(t)
	ctx := context.Background()
	assert.ErrorIs(t, m.SendToBin(ctx, BinAddress{Layer: 1, Bin: 40}), ErrNoSuchBin)
	assert.ErrorIs(t, m.SendToBin(ctx, BinAddress{Layer: 7, Bin: 1}), ErrNoSuchBin)
	assert.ErrorIs(t, m.TestLayer(ctx, 7, 0), ErrNoSuchBin)
}

func TestMachineTestLayer(t *testing.T) {
	m, hw := homedMachine(t)
	require.NoError(t, m.TestLayer(context.Background(), 4, time.Millisecond))
	last := waitForJob(t, m)
	assert.Empty(t, last.Error)
	assert.Equal(t, 12, last.Step)
	assert.Equal(t, 12, last.Total)
	assert.Equal(t, -1, hw.Layer(), "every layer closed afterwards")

	bins := DefaultMachineSettings().Distributor.LayerBins()[3]
	assert.InDelta(t, bins[11].Angle, hw.Position(ChannelDistributor), 1e-9, "ends on the last bin")
}

func TestMachineJobsAreExclusiveAndCancellable(t *testing.T) {
	m, _ := homedMachine(t)
	ctx := context.Background()
	require.NoError(t, m.TestLayer(ctx, 1, time.Hour))
	require.Eventually(t, func() bool { return m.Snapshot().Job != nil }, time.Second, 5*time.Millisecond)

	assert.ErrorIs(t, m.SendToBin(ctx, BinAddress{Layer: 1, Bin: 1}), ErrBusy)
	assert.ErrorIs(t, m.MoveChannel(ctx, ChannelC4, 72, 10), ErrBusy)
	assert.ErrorIs(t, do(t, m, CommandStart), ErrBusy)

	require.NoError(t, m.CancelJob(ctx))
	assert.Equal(t, "Cancelled", waitForJob(t, m).Error)
	assert.Equal(t, StateReady, m.Snapshot().State, "cancelling a test leaves the machine ready")
}

func TestMachineHomingCanBeCancelled(t *testing.T) {
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.DoorSettleMs = 60_000 // homing hangs on the servo wait
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	m := NewMachine(hw, settings, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateHoming)
	require.NoError(t, m.CancelJob(context.Background()))
	waitForState(t, m, StateError)
	assert.Contains(t, m.Snapshot().Error, "canceled")
}

func TestMachineManualMove(t *testing.T) {
	m, hw := startMachine(t)
	ctx := context.Background()
	require.NoError(t, m.MoveChannel(ctx, ChannelC4, 72, 10))
	assert.Equal(t, 72.0, hw.Position(ChannelC4))

	require.NoError(t, do(t, m, CommandHome))
	waitForState(t, m, StateReady)
	require.NoError(t, do(t, m, CommandStart))
	assert.ErrorIs(t, m.MoveChannel(ctx, ChannelC4, 72, 10), ErrInvalidTransition)
	assert.Equal(t, 72.0, hw.Position(ChannelC4))
}

func TestMachineSubscribersSeeChanges(t *testing.T) {
	m, _ := startMachine(t)
	updates, stop := m.Subscribe()
	defer stop()

	require.NoError(t, do(t, m, CommandHome))
	deadline := time.After(time.Second)
	for {
		select {
		case snap := <-updates:
			if snap.State == StateReady {
				return
			}
		case <-deadline:
			t.Fatal("no ready snapshot")
		}
	}
}
