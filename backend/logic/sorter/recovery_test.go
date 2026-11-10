package sorter

import (
	"math"
	"testing"

	"goklippy/backend/mcu"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotorShake(t *testing.T) {
	p, err := shakeProfile("heavy")
	require.NoError(t, err)
	sh := rotorShake(p, 10.8) // C4's gearing
	assert.InDelta(t, 6/10.8, sh.AmplitudeDeg, 1e-9)
	assert.InDelta(t, 5000.0/3200*60/10.8, sh.Rpm, 1e-9)
	assert.Equal(t, 12, sh.Cycles)

	_, err = shakeProfile("wobble")
	assert.ErrorIs(t, err, ErrUnknownShake)
}

// Every shake goes out in parts of whole cycles, so at least one cycle must fit the
// MCU's command budget on every channel's gearing.
func TestShakesFitTheMCU(t *testing.T) {
	settings := DefaultMachineSettings()
	for _, ch := range []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4} {
		gear := channelGear(settings, ch)
		stepsPerRev := float64(fullStepsPerRev*defaultMicrostep) * gear
		for _, p := range ShakeProfiles {
			sh := rotorShake(p, gear)
			steps := int(math.Round(sh.AmplitudeDeg / 360 * stepsPerRev))
			require.GreaterOrEqual(t, steps, 1, "%s %s", ch, p.Key)
			n, err := mcu.ShakeQueueOps(mcu.TrapezoidMove{
				Steps: steps, MaxHz: sh.Rpm / 60 * stepsPerRev, AccelHz: sh.AccelRpmS / 60 * stepsPerRev,
			}, 2, 16e6)
			require.NoError(t, err, "%s %s", ch, p.Key)
			assert.LessOrEqual(t, n, mcu.MaxShakeQueueOps, "%s %s", ch, p.Key)
		}
	}
}
