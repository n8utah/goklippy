package sorter

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptVision shows C4 with whatever pieces the test puts there, a new frame (and
// observation of it) each time it's asked.
type scriptVision struct {
	mu     sync.Mutex
	pieces []vision.PieceObs
	merged []vision.MergedBox
	at     time.Time
}

func (v *scriptVision) set(at time.Time, pieces ...vision.PieceObs) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pieces, v.at = pieces, at
}

func (v *scriptVision) Observation(Channel) vision.Observation {
	v.mu.Lock()
	defer v.mu.Unlock()
	o := vision.Observation{Valid: true, At: v.at, Pieces: v.pieces, Merged: v.merged}
	for _, p := range v.pieces {
		if p.Zone == vision.ZoneDrop {
			o.InDrop = true
		}
	}
	return o
}

func (v *scriptVision) Frame(Channel) (image.Image, time.Time, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return image.NewRGBA(image.Rect(0, 0, 640, 480)), v.at, true
}

func dropAt(x int) vision.PieceObs {
	return vision.PieceObs{Zone: vision.ZoneDrop, Box: vision.Box{X1: x, Y1: 100, X2: x + 40, Y2: 140}}
}

func captureTestPlatter(t *testing.T) (*Platter, *scriptVision) {
	t.Helper()
	v := &scriptVision{}
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Distributor.LeftOut = true
		s.Platter.SettleMs, s.Platter.BurstFrames = 0, 3
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	p := NewPlatter(PlatterDeps{HW: hw, Vision: v, Settings: settings, Records: NewRecords(nil, t.TempDir()), Identifier: noIdentifier{}})
	return p, v
}

type noIdentifier struct{}

func (noIdentifier) Identify(context.Context, [][]byte) (brickognize.Result, error) {
	return brickognize.Result{}, nil
}

// step shows the platter one more frame, 100 ms after the last.
func step(p *Platter, v *scriptVision, t *time.Time, pieces ...vision.PieceObs) {
	*t = t.Add(100 * time.Millisecond)
	v.set(*t, pieces...)
	p.Step(*t)
}

func TestPlatterPhotographsOnlyOnceThePieceIsStill(t *testing.T) {
	p, v := captureTestPlatter(t)
	now := time.Now()
	p.settleUntil = now

	// Lands, then slides 30 px, 20 px...
	for _, x := range []int{100, 130, 150} {
		step(p, v, &now, dropAt(x))
	}
	s := p.slots[0]
	require.NotNil(t, s)
	assert.Empty(t, s.shots, "no photo while it slides")
	// ...then lies still: photos start, all of it where it came to rest.
	for range 4 {
		step(p, v, &now, dropAt(152))
	}
	require.NotEmpty(t, s.shots)
	for _, sh := range s.shots {
		assert.Equal(t, 152, sh.box.X1)
	}
}

func TestPlatterStartsOverWhenThePieceIsBumped(t *testing.T) {
	p, v := captureTestPlatter(t)
	now := time.Now()
	p.settleUntil = now
	for range 3 { // still for 2 frames, then 2 of the 3 photos
		step(p, v, &now, dropAt(100))
	}
	s := p.slots[0]
	require.NotEmpty(t, s.shots)
	require.Equal(t, slotCapturing, s.state)
	step(p, v, &now, dropAt(160)) // knocked along
	assert.Empty(t, s.shots, "photos of it in the old place are dropped")
	assert.Equal(t, slotSettling, s.state)
}

// A landed piece that slides just out of the drop sector is still photographed, at
// worst where it was last seen once settleMax has passed: C4 never waits for it forever.
func TestPlatterDoesntWaitForeverForAPieceOutOfTheDropSector(t *testing.T) {
	p, v := captureTestPlatter(t)
	now := time.Now()
	p.settleUntil = now
	step(p, v, &now, dropAt(100))
	s := p.slots[0]
	require.NotNil(t, s)
	far := vision.PieceObs{Zone: vision.ZoneNone, Box: vision.Box{X1: 500, Y1: 400, X2: 540, Y2: 440}}
	for range 25 { // 2.5 s with it nowhere near
		step(p, v, &now, far)
	}
	assert.NotEqual(t, slotSettling, s.state, "photographed anyway after settleMax")
	assert.NotEmpty(t, s.shots)
}

func TestPlatterCallsASecondPieceInTheDropSectorADouble(t *testing.T) {
	p, v := captureTestPlatter(t)
	now := time.Now()
	p.settleUntil = now
	step(p, v, &now, dropAt(100))
	step(p, v, &now, dropAt(100), dropAt(300)) // a second one bounces in
	s := p.slots[0]
	assert.Equal(t, PieceMultiDrop, s.piece.Status)
	assert.Equal(t, slotResolved, s.state)
}

func TestPlatterCallsALandingADoubleWhenC3DroppedTwo(t *testing.T) {
	p, v := captureTestPlatter(t)
	now := time.Now()
	p.settleUntil = now
	// C3's news comes just after the landing...
	step(p, v, &now, dropAt(100))
	p.ExpectSeveral(now)
	assert.Equal(t, PieceMultiDrop, p.slots[0].piece.Status)

	// ...or just before it.
	p, v = captureTestPlatter(t)
	p.settleUntil = now
	p.ExpectSeveral(now)
	step(p, v, &now, dropAt(100))
	assert.Equal(t, PieceMultiDrop, p.slots[0].piece.Status)
}

func TestLinedUpOnlyForAMuchSmallerPart(t *testing.T) {
	torso := vision.Box{X1: 100, Y1: 100, X2: 140, Y2: 140}
	head := vision.Box{X1: 110, Y1: 80, X2: 130, Y2: 98}
	plate := vision.Box{X1: 100, Y1: 60, X2: 140, Y2: 98}
	assert.True(t, linedUp(torso, head), "a minifigure's head over its torso")
	assert.False(t, linedUp(torso, plate), "two pieces the same size end to end")
}

func TestFeederNotesTwoFallingTogether(t *testing.T) {
	f := NewFeeder()
	s := feederSettings()
	t0 := time.Now()
	two := obs(t0, piece(vision.ZoneExit, -1), piece(vision.ZonePrecise, 3))
	f.Step(FeederInput{Now: t0, C2: obs(t0), C3: two, PlatterReady: true}, s)
	t1 := t0.Add(300 * time.Millisecond)
	f.Step(FeederInput{Now: t1, C2: obs(t1), C3: obs(t1), PlatterReady: true}, s)
	assert.True(t, f.TakeSeveralFell(ChannelC3), "both went")
	assert.False(t, f.TakeSeveralFell(ChannelC3), "reported once")

	f = NewFeeder()
	f.Step(FeederInput{Now: t0, C2: obs(t0), C3: two, PlatterReady: true}, s)
	f.Step(FeederInput{Now: t1, C2: obs(t1), C3: obs(t1, piece(vision.ZonePrecise, 3)), PlatterReady: true}, s)
	assert.False(t, f.TakeSeveralFell(ChannelC3), "one went")
}
