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

func TestPlatterFollowsAPieceLaggingAtTheExit(t *testing.T) {
	p, v, hw := dropTestPlatter(t)
	var didntFall int
	p.deps.OnDidntFall = func() { didntFall++ }
	settings := p.deps.Settings()
	exit := p.exitIndex(settings) - 1
	s := &platterSlot{piece: Piece{ID: "p1", Status: PieceIdentified}, state: slotResolved, angle: 170, routed: true}
	p.slots[exit] = s
	p.aimedFor = s.piece.ID
	z := settings.Cameras.Zones["c4"]
	at := func(angle float64) vision.PieceObs {
		return vision.PieceObs{Angle: angle, ForwardToExit: z.ForwardToExit(angle)}
	}
	now := p.settleUntil
	// stepUntil steps (the simulated move reports done on its own goroutine) until
	// there have been n moves.
	stepUntil := func(n int, pieces ...vision.PieceObs) {
		for i := 0; i < 20 && len(hw.Moves()) < n; i++ {
			time.Sleep(time.Millisecond)
			step(p, v, &now, pieces...)
		}
	}

	step(p, v, &now, at(262))
	require.Len(t, hw.Moves(), 1)
	extra := hw.Moves()[0].Degrees - 72

	stepUntil(2, at(10)) // it dragged on the wall: 14° short of the exit
	require.Len(t, hw.Moves(), 2, "turns on after it")
	assert.InDelta(t, 34.5, hw.Moves()[1].Degrees, 0.01, "as far as the exit's middle")

	stepUntil(3) // now it's gone
	require.Len(t, hw.Moves(), 3)
	assert.InDelta(t, -(extra + 34.5), hw.Moves()[2].Degrees, 0.01, "back by everything past the sector")
	assert.Zero(t, didntFall, "it fell: no recovery")
}

func TestPlatterLeavesAnotherSlotsPieceAlone(t *testing.T) {
	p, _, _ := dropTestPlatter(t)
	settings := p.deps.Settings()
	z := settings.Cameras.Zones["c4"]
	p.slots[1] = &platterSlot{angle: 300} // landed at 300°: one sector on, at 12° now
	o := vision.PieceObs{Angle: 12, ForwardToExit: z.ForwardToExit(12)}
	_, there := p.straggler(vision.Observation{Valid: true, Pieces: []vision.PieceObs{o}}, settings)
	assert.False(t, there, "it's where its slot says: not a piece left behind")
	assert.InDelta(t, 9, p.followTurn(vision.PieceObs{Angle: 5}, settings), 0.01,
		"a turn after a piece lagging behind it stops it 3° short of the exit")
}
