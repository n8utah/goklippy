package sorter

import (
	"image"
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stillVision sees C4 valid and empty, with no frames to photograph.
type stillVision struct{}

func (stillVision) Observation(Channel) vision.Observation {
	return vision.Observation{Valid: true, At: time.Now().Add(-time.Hour)}
}
func (stillVision) Frame(Channel) (image.Image, time.Time, bool) { return nil, time.Time{}, false }

func exitTestPlatter(t *testing.T, clearExitFirst bool) (*Platter, *SimHardware) {
	t.Helper()
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.LeftOut = true // no chute to aim: only the platter's timing matters
		s.Platter.ClearExitFirst = clearExitFirst
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	p := NewPlatter(PlatterDeps{
		HW: hw, Vision: stillVision{}, Settings: settings,
		Records: NewRecords(nil, t.TempDir()),
	})
	return p, hw
}

func c4Moves(hw *SimHardware) int {
	n := 0
	for _, mv := range hw.Moves() {
		if mv.Channel == ChannelC4 {
			n++
		}
	}
	return n
}

// exitSlot puts a piece in the slot that falls on the next index.
func exitSlot(p *Platter, state slotState) *platterSlot {
	s := &platterSlot{piece: Piece{ID: NewPieceID(), Status: PieceIdentified}, state: state}
	p.slots[p.exitIndex(p.deps.Settings())-1] = s
	return s
}

func TestPlatterDropsAReadyPieceAtOnce(t *testing.T) {
	p, hw := exitTestPlatter(t, true)
	now := time.Now()
	p.settleUntil, p.freeSince = now, now // the drop sector has only just come free
	exitSlot(p, slotResolved)

	p.Step(now)
	assert.Equal(t, 1, c4Moves(hw), "no idle wait: the identified piece drops now")
}

func TestPlatterWaitsOutTheIdleTimeWithoutClearExitFirst(t *testing.T) {
	p, hw := exitTestPlatter(t, false)
	now := time.Now()
	p.settleUntil, p.freeSince = now, now
	exitSlot(p, slotResolved)

	p.Step(now)
	assert.Equal(t, 0, c4Moves(hw), "the old way: wait IdleIndexMs")
	p.Step(now.Add(2 * time.Second))
	assert.Equal(t, 1, c4Moves(hw))
}

func TestPlatterTakesASecondPieceOnlyBehindOneAtTheExit(t *testing.T) {
	p, _ := exitTestPlatter(t, true)
	now := time.Now()
	require.True(t, p.Ready(now), "empty: C3 may drop")

	s := exitSlot(p, slotIdentifying)
	assert.True(t, p.Ready(now), "one at the exit, still being identified: a second may land")
	s.state = slotResolved
	assert.False(t, p.Ready(now), "identified and about to fall: nothing lands until it has")

	p.slots[p.exitIndex(p.deps.Settings())-1] = nil
	assert.True(t, p.Ready(now), "once it's gone, C3 may drop again")

	// A piece on its way round, not yet at the exit, leaves no room.
	p.deps.Settings = func() MachineSettings {
		st := DefaultMachineSettings()
		st.Distributor.LeftOut, st.Platter.ExitSector = true, 3
		return st
	}
	p.slots[1] = &platterSlot{piece: Piece{ID: NewPieceID()}, state: slotIdentifying}
	assert.False(t, p.Ready(now), "two pieces only when the other is at the exit")
}

func TestPlatterLetsC3DropWithoutClearExitFirst(t *testing.T) {
	p, _ := exitTestPlatter(t, false)
	exitSlot(p, slotIdentifying)
	assert.True(t, p.Ready(time.Now()))
}
