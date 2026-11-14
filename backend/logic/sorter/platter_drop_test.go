package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropTestPlatter is a platter with C4's exit at 24–65° (its middle 44.5°), seeing
// what v is set to.
func dropTestPlatter(t *testing.T) (*Platter, *scriptVision, *SimHardware) {
	t.Helper()
	v := &scriptVision{}
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.LeftOut = true
		s.Platter.SettleMs, s.Platter.FallWaitMs = 0, 0
		s.Cameras.Zones = map[string]vision.ChannelZones{"c4": {Exit: vision.Span{Start: 24, End: 65}}}
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	p := NewPlatter(PlatterDeps{HW: hw, Vision: v, Settings: settings, Records: NewRecords(nil, t.TempDir())})
	return p, v, hw
}

func seenAt(angle float64) vision.PieceObs { return vision.PieceObs{Angle: angle} }

func TestDropExtraBringsThePieceToTheMiddleOfTheExit(t *testing.T) {
	p, _, _ := dropTestPlatter(t)
	settings := p.deps.Settings()
	exit := p.exitIndex(settings) - 1
	s := &platterSlot{angle: 170} // landed at 170°, one sector ago: about 242° now
	p.slots[exit] = s
	obs := func(ps ...vision.PieceObs) vision.Observation { return vision.Observation{Valid: true, Pieces: ps} }

	assert.InDelta(t, 70.5, p.dropExtra(obs(seenAt(262)), settings), 0.01,
		"a sector leaves it at 334°: 70.5° more to the exit's middle")
	assert.InDelta(t, 72, p.dropExtra(obs(seenAt(220)), settings), 0.01, "at most a sector")
	assert.InDelta(t, 9, p.dropExtra(obs(seenAt(262), seenAt(300)), settings), 0.01,
		"the piece ahead of it stops 3° short of the exit")
	assert.Zero(t, p.dropExtra(obs(seenAt(340)), settings), "nothing seen near where it should be")
	assert.Zero(t, p.dropExtra(obs(), settings), "nothing seen")

	s.angle = 278 // now about 350°: a sector takes it to 62°, past the middle
	assert.Zero(t, p.dropExtra(obs(seenAt(350)), settings), "already far enough")
}

func TestPlatterTurnsBackAfterDroppingWithExtra(t *testing.T) {
	p, v, hw := dropTestPlatter(t)
	settings := p.deps.Settings()
	exit := p.exitIndex(settings) - 1
	s := &platterSlot{piece: Piece{ID: "p1", Status: PieceIdentified}, state: slotResolved, angle: 170, routed: true}
	p.slots[exit] = s
	p.aimedFor = s.piece.ID
	now := p.settleUntil

	step(p, v, &now, seenAt(262))
	require.Len(t, hw.Moves(), 1)
	assert.InDelta(t, 72+70.5, hw.Moves()[0].Degrees, 0.01, "on past the sector to drop it")
	assert.False(t, p.Ready(now), "C3 doesn't drop while the sectors are out")

	// The index finishes, the piece has fallen, its fall is judged: turn back.
	for i := 0; i < 10 && len(hw.Moves()) < 2; i++ {
		time.Sleep(time.Millisecond) // the simulated move reports done on its own goroutine
		step(p, v, &now)
	}
	require.Len(t, hw.Moves(), 2)
	assert.InDelta(t, -70.5, hw.Moves()[1].Degrees, 0.01, "back as far, so the sectors are where the slots are")
	for i := 0; i < 10 && !p.Ready(now); i++ {
		time.Sleep(time.Millisecond)
		step(p, v, &now)
	}
	assert.True(t, p.Ready(now), "then C3 may drop again")
	assert.Equal(t, 1, p.Delivered())
}
