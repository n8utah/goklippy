package sorter

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRound records the angle each picture was taken at.
type fakeRound struct {
	hw      *SimHardware
	mu      sync.Mutex
	angles  []float64
	set     [][]byte
	spacing float64
}

func (f *fakeRound) ChannelFrameAfter(ctx context.Context, ch Channel, t time.Time) (image.Image, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.angles = append(f.angles, f.hw.Position(ch))
	return image.NewGray(image.Rect(0, 0, 4, 4)), []byte{byte(len(f.angles))}, nil
}

func (f *fakeRound) SetViews(ch Channel, jpegs [][]byte, spacing float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.set, f.spacing = jpegs, spacing
	return nil
}

func TestCaptureViewsCoversOneSection(t *testing.T) {
	m, hw := startMachine(t) // C3 has 10 sections: its look repeats every 36°
	f := &fakeRound{hw: hw}
	require.NoError(t, m.CaptureViews(context.Background(), ChannelC3, f, 6))
	require.Eventually(t, func() bool { return m.Snapshot().Job == nil }, 30*time.Second, 10*time.Millisecond)
	require.Empty(t, m.Snapshot().LastJob.Error)
	assert.Contains(t, m.Snapshot().LastJob.Name, "across a section (36°)")

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, []float64{0, 6, 12, 18, 24, 30}, f.angles)
	assert.Len(t, f.set, 6)
	assert.Equal(t, 6.0, f.spacing)
	assert.InDelta(t, 36, hw.Position(ChannelC3), 1e-9, "one section")
}

func TestCaptureViewsGoesAllTheWayRoundWithoutSections(t *testing.T) {
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		c3 := s.Channels["c3"]
		c3.Sections = 0
		s.Channels = map[string]ChannelSettings{"c1": s.Channels["c1"], "c2": s.Channels["c2"], "c3": c3, "c4": s.Channels["c4"]}
		return s
	}
	hw := NewSimHardware(settings)
	hw.TimeScale = 0
	m := NewMachine(hw, settings, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	f := &fakeRound{hw: hw}
	require.NoError(t, m.CaptureViews(context.Background(), ChannelC3, f, 30))
	require.Eventually(t, func() bool { return m.Snapshot().Job == nil }, 30*time.Second, 10*time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Len(t, f.set, 12)
	assert.InDelta(t, 360, hw.Position(ChannelC3), 1e-9, "a whole turn")
}

func TestRotorPeriod(t *testing.T) {
	s := DefaultMachineSettings()
	assert.Equal(t, 36.0, s.RotorPeriod(ChannelC3))
	assert.Equal(t, 72.0, s.RotorPeriod(ChannelC4), "the platter's 5 sectors")
}

func TestCaptureViewsChecksItsInput(t *testing.T) {
	m, hw := startMachine(t)
	f := &fakeRound{hw: hw}
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC1, f, 2), ErrNotCameraCh)
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC3, f, 0.1), ErrInvalidSettings)
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC3, f, 45), ErrInvalidSettings)
}
