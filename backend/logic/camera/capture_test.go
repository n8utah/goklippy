package camera

import (
	"bytes"
	"context"
	"image/jpeg"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCaptureFromFfmpegTestSource runs the real capture against ffmpeg's built-in
// test picture. It needs ffmpeg on PATH, so it's skipped without one.
func TestCaptureFromFfmpegTestSource(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	cam := New(Config{Device: "lavfi:testsrc", Width: 320, Height: 240, Fps: 10})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames, stop := cam.Subscribe()
	defer stop()
	go cam.Run(ctx)

	var got []Frame
	deadline := time.After(10 * time.Second)
	for len(got) < 5 {
		select {
		case f := <-frames:
			got = append(got, f)
		case <-deadline:
			t.Fatalf("only %d frames in 10 s; status %+v", len(got), cam.Status())
		}
	}
	img, err := jpeg.Decode(bytes.NewReader(got[len(got)-1].JPEG))
	require.NoError(t, err)
	assert.Equal(t, 320, img.Bounds().Dx())
	assert.Equal(t, 240, img.Bounds().Dy())
	assert.Greater(t, got[len(got)-1].Seq, got[0].Seq)
	assert.True(t, cam.Status().Running)

	cancel()
	require.Eventually(t, func() bool { return !cam.Status().Running }, 5*time.Second, 20*time.Millisecond)
}
