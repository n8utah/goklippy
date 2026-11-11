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
	hw     *SimHardware
	mu     sync.Mutex
	angles []float64
	set    [][]byte
}

func (f *fakeRound) ChannelFrameAfter(ctx context.Context, ch Channel, t time.Time) (image.Image, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.angles = append(f.angles, f.hw.Position(ch))
	return image.NewGray(image.Rect(0, 0, 4, 4)), []byte{byte(len(f.angles))}, nil
}

func (f *fakeRound) SetViews(ch Channel, jpegs [][]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.set = jpegs
	return nil
}

func TestCaptureViewsGoesAllTheWayRound(t *testing.T) {
	m, hw := startMachine(t)
	f := &fakeRound{hw: hw}
	require.NoError(t, m.CaptureViews(context.Background(), ChannelC3, f, 30))
	require.Eventually(t, func() bool { return m.Snapshot().Job == nil }, 30*time.Second, 10*time.Millisecond)
	require.Empty(t, m.Snapshot().LastJob.Error)

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, []float64{0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330}, f.angles)
	assert.Len(t, f.set, 12)
	assert.InDelta(t, 360, hw.Position(ChannelC3), 1e-9, "a whole turn: back where it began")
}

func TestCaptureViewsChecksItsInput(t *testing.T) {
	m, hw := startMachine(t)
	f := &fakeRound{hw: hw}
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC1, f, 2), ErrNotCameraCh)
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC3, f, 0.1), ErrInvalidSettings)
	assert.ErrorIs(t, m.CaptureViews(context.Background(), ChannelC3, f, 45), ErrInvalidSettings)
}
