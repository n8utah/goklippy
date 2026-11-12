package sorter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearMoves is how far each channel turned, by channel.
func clearMoves(hw *SimHardware) map[Channel]SimMove {
	out := map[Channel]SimMove{}
	for _, mv := range hw.Moves() {
		if mv.Channel != ChannelDistributor {
			out[mv.Channel] = mv
		}
	}
	return out
}

func TestClearChannelsWithTheChuteIn(t *testing.T) {
	m, hw := homedMachine(t)
	require.NoError(t, hw.SelectLayer(1))

	require.NoError(t, m.ClearChannels(context.Background()))
	last := waitForJob(t, m)
	assert.Equal(t, "Clear C2–C4", last.Name)
	assert.Empty(t, last.Error)
	assert.Equal(t, -1, hw.Layer(), "every layer closed, so it all drops into the bucket")

	s := DefaultMachineSettings()
	moves := clearMoves(hw)
	require.Len(t, moves, 3, "C2, C3 and C4, not C1")
	rpm := s.Feeder.ClearRpm
	assert.Greater(t, rpm, s.FeederRpm["c3"], "faster than sorting")
	assert.Greater(t, rpm, s.IndexRpm, "faster than sorting")
	for _, ch := range clearChannels {
		mv := moves[ch]
		assert.Equal(t, rpm, mv.Rpm, "%s at the one clear speed", ch)
		assert.Equal(t, s.Feeder.ClearAccel, mv.Accel, "%s ramps up", ch)
		took := moveDuration(mv.Degrees, mv.Rpm, mv.Accel).Seconds()
		assert.InDelta(t, float64(s.Feeder.ClearSeconds), took, 0.01, "%s for the set time, ramps included", ch)
	}
}

func TestClearChannelsWithTheChuteLeftOut(t *testing.T) {
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.LeftOut = true
		s.Feeder.ClearSeconds = 4
		s.Feeder.ClearRpm = 20
		s.Feeder.ClearAccel = 0
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	require.NoError(t, hw.SelectLayer(2))
	m := NewMachine(hw, settings, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go m.Run(ctx)

	require.NoError(t, m.ClearChannels(context.Background()))
	assert.Empty(t, waitForJob(t, m).Error)
	assert.Equal(t, 2, hw.Layer(), "the chute's layers aren't touched")
	moves := clearMoves(hw)
	require.Len(t, moves, 3)
	for _, ch := range clearChannels {
		assert.InDelta(t, 20*360*4/60.0, moves[ch].Degrees, 1e-9, "%s: 20 rpm for 4 s, no ramp", ch)
	}
}

func TestClearChannelsIsRefusedWhileBusy(t *testing.T) {
	m, hw := homedMachine(t)
	hw.TimeScale = 1
	require.NoError(t, m.ClearChannels(context.Background()))
	assert.ErrorIs(t, m.ClearChannels(context.Background()), ErrBusy)
	require.NoError(t, m.CancelJob(context.Background()))
	assert.Equal(t, "Cancelled", waitForJob(t, m).Error)
}
