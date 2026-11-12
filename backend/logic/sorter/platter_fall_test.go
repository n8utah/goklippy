package sorter

import (
	"image"
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
)

// exitVision shows C4 with or without a piece in its exit.
type exitVision struct {
	inExit bool
	at     time.Time
}

func (v *exitVision) Observation(Channel) vision.Observation {
	o := vision.Observation{Valid: true, At: v.at, InExit: v.inExit}
	if v.inExit {
		o.Pieces = []vision.PieceObs{{Zone: vision.ZoneExit}}
	}
	return o
}
func (v *exitVision) Frame(Channel) (image.Image, time.Time, bool) { return nil, time.Time{}, false }

func fallTestPlatter(t *testing.T) (*Platter, *exitVision, *int) {
	t.Helper()
	v := &exitVision{}
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.LeftOut = true
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	didnt := 0
	p := NewPlatter(PlatterDeps{HW: hw, Vision: v, Settings: settings, Records: NewRecords(nil, t.TempDir()),
		OnDidntFall: func() { didnt++ }})
	return p, v, &didnt
}

// A piece just dropped: the platter watches the exit until it's clear.
func dropped(p *Platter, now time.Time) {
	p.recent = []Piece{{ID: "fell"}}
	p.settleUntil = now
	p.checkFall = true
	p.fallBy = now.Add(time.Duration(DefaultMachineSettings().Platter.FallWaitMs) * time.Millisecond)
}

func TestPlatterLetsAPieceSlideOffBeforeCallingItStuck(t *testing.T) {
	p, v, didnt := fallTestPlatter(t)
	now := time.Now()
	dropped(p, now)
	for _, ms := range []int{50, 200, 400} { // still in the exit, sliding
		v.at, v.inExit = now.Add(time.Duration(ms)*time.Millisecond), true
		p.Step(v.at)
	}
	v.at, v.inExit = now.Add(600*time.Millisecond), false // gone
	p.Step(v.at)
	assert.Zero(t, *didnt)
	assert.False(t, p.checkFall)
}

func TestPlatterCallsAPieceThatStaysNotFallen(t *testing.T) {
	p, v, didnt := fallTestPlatter(t)
	now := time.Now()
	dropped(p, now)
	v.at, v.inExit = now.Add(100*time.Millisecond), true
	p.Step(v.at)
	assert.Zero(t, *didnt, "too soon to tell")
	v.at = now.Add(time.Second)
	p.Step(v.at)
	assert.Equal(t, 1, *didnt, "still there after the wait")
}
