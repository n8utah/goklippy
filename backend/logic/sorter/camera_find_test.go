package sorter

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCameras draws, for each device, a bar that moves with the channel it watches.
type fakeCameras struct {
	hw      *SimHardware
	watches map[string]Channel // device -> channel; absent watches nothing that moves
}

func (f fakeCameras) FrameAfter(ctx context.Context, device string, t time.Time) (image.Image, []byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 160, 120))
	for i := range img.Pix {
		img.Pix[i] = 40
	}
	if ch, ok := f.watches[device]; ok {
		x0 := 50 + int(f.hw.Position(ch)*2)%60 // in the middle; 15° moves it 30 px
		for y := 30; y < 90; y++ {
			for x := x0; x < x0+20 && x < 160; x++ {
				img.Set(x, y, color.RGBA{220, 220, 220, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return img, buf.Bytes(), nil
}

func runFind(t *testing.T, m *Machine, src FrameSource, devices []string) (CameraFindResult, *JobStatus) {
	t.Helper()
	finder := NewCameraFinder()
	require.NoError(t, m.FindCameras(context.Background(), src, finder, CameraFindRequest{Devices: devices}))
	require.Eventually(t, func() bool { return m.Snapshot().Job == nil }, 5*time.Second, 5*time.Millisecond)
	r, err := finder.Last()
	require.NoError(t, err)
	return r, m.Snapshot().LastJob
}

func TestFindCamerasProposesWhichIsWhere(t *testing.T) {
	m, hw := startMachine(t)
	src := fakeCameras{hw: hw, watches: map[string]Channel{"camA": ChannelC3, "camB": ChannelC2, "camC": ChannelC4}}

	r, job := runFind(t, m, src, []string{"laptop", "camA", "camB", "camC"})
	assert.Empty(t, job.Error)
	assert.Equal(t, map[Channel]string{ChannelC2: "camB", ChannelC3: "camA", ChannelC4: "camC"}, r.Proposed)
	assert.Zero(t, r.Scores[ChannelC3]["laptop"], "the laptop camera saw nothing move")
	assert.Positive(t, r.Scores[ChannelC3]["camA"])
	assert.Empty(t, r.Notes)

	for _, ch := range []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4} {
		assert.InDelta(t, 0, hw.Position(ch), 1e-9, "%s is turned back", ch)
	}
	finder := NewCameraFinder()
	_, err := finder.Image(ChannelC2, false)
	assert.ErrorIs(t, err, ErrNotFinding)
}

func TestFindCamerasNotesAChannelNobodySees(t *testing.T) {
	m, hw := startMachine(t)
	src := fakeCameras{hw: hw, watches: map[string]Channel{"camA": ChannelC2, "camB": ChannelC3}}

	r, job := runFind(t, m, src, []string{"camA", "camB"})
	assert.Empty(t, job.Error)
	assert.Equal(t, map[Channel]string{ChannelC2: "camA", ChannelC3: "camB"}, r.Proposed)
	require.Len(t, r.Notes, 1)
	assert.Contains(t, r.Notes[0], "No camera saw C4 move")
}

func TestFindCamerasSaysWhenNothingMoves(t *testing.T) {
	m, hw := startMachine(t)
	src := fakeCameras{hw: hw} // the motors "turn" but no camera sees anything move

	r, job := runFind(t, m, src, []string{"camA", "camB"})
	assert.Contains(t, job.Error, "motors powered")
	assert.Contains(t, r.Error, "motors powered")
	assert.Empty(t, r.Proposed)
}

func TestFindCamerasNeedsCamerasAndAFreeMachine(t *testing.T) {
	m, hw := startMachine(t)
	src := fakeCameras{hw: hw}
	assert.ErrorIs(t, m.FindCameras(context.Background(), src, NewCameraFinder(), CameraFindRequest{}), ErrInvalidSettings)

	hw.SetConnected(false)
	require.Eventually(t, func() bool { return !m.Snapshot().Hardware.Connected }, 3*time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, m.FindCameras(context.Background(), src, NewCameraFinder(), CameraFindRequest{Devices: []string{"x"}}), ErrNoBoard)
}

func TestChangedShare(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 80, 80))
	b := image.NewRGBA(image.Rect(0, 0, 80, 80))
	assert.Zero(t, changedShare(a, b))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			b.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	assert.InDelta(t, 0.5, changedShare(a, b), 0.01, "the top half of the middle changed")
	assert.Equal(t, 1.0, changedShare(a, image.NewRGBA(image.Rect(0, 0, 20, 20))))
}

func TestChangedShareIgnoresTheEdges(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 40, 40))
	b := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 8; x++ { // a neighboring channel moving in the left edge
			b.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	assert.Zero(t, changedShare(a, b))
}
