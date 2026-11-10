package sorter

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerceptionWithFfmpegTestPicture runs the camera, detector and zones on ffmpeg's
// plain gray test picture. It needs ffmpeg on PATH.
func TestPerceptionWithFfmpegTestPicture(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	var mu sync.Mutex
	current := DefaultMachineSettings()
	current.Cameras.Devices = map[string]string{"c4": "lavfi:color=c=gray"}
	settings := func() MachineSettings {
		mu.Lock()
		defer mu.Unlock()
		return current
	}
	dir := t.TempDir()
	p := NewPerception(settings, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	require.Eventually(t, func() bool { _, _, ok := p.JPEG(ChannelC4); return ok }, 10*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return p.Statuses()[ChannelC4].Message != "" }, 5*time.Second, 20*time.Millisecond)
	st := p.Statuses()[ChannelC4]
	assert.True(t, st.Camera.Running)
	assert.Equal(t, 640, st.FrameW)
	assert.Contains(t, st.Message, "zones", "no zones yet")
	assert.False(t, p.Observation(ChannelC4).Valid)

	mu.Lock()
	current.Cameras.Zones = map[string]vision.ChannelZones{"c4": SimZones()}
	mu.Unlock()
	require.Eventually(t, func() bool { return p.Statuses()[ChannelC4].Message == vision.ErrNoBackground.Error() }, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, p.CaptureBackground(ChannelC4))
	require.Eventually(t, func() bool { return p.Observation(ChannelC4).Valid }, 5*time.Second, 20*time.Millisecond)
	assert.True(t, p.Observation(ChannelC4).Empty(), "an empty gray channel")
	assert.True(t, p.Statuses()[ChannelC4].HasBackground)
	assert.FileExists(t, p.backgroundPath(ChannelC4))

	assert.ErrorIs(t, p.CaptureBackground(ChannelC2), ErrNoCamera)

	// A restart picks the saved background back up.
	cancel()
	p2 := NewPerception(settings, dir)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	go p2.Run(ctx2)
	require.Eventually(t, func() bool { return p2.Observation(ChannelC4).Valid }, 10*time.Second, 20*time.Millisecond)
}
