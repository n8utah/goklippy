package sorter

import (
	"context"
	"testing"
	"time"

	"goklippy/backend/logic/camera"

	"github.com/stretchr/testify/assert"
)

func TestCameraPoolSharesOneCapturePerDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newCameraPool(ctx)
	defer p.stopAll()
	cfg := camera.Config{Device: TestDevice, Width: 64, Height: 48, Fps: 5}

	preview := p.preview(cfg)
	assert.Same(t, preview, p.acquire(cfg), "a channel takes over the capture being previewed")
	assert.Same(t, preview, p.preview(cfg), "and a preview of an assigned camera watches the channel's capture")

	p.release(TestDevice)
	assert.NotNil(t, p.running(TestDevice), "still previewed")
	p.sweep(time.Now().Add(previewHold + time.Second))
	assert.Nil(t, p.running(TestDevice), "stopped once nobody uses or watches it")
}

func TestCameraPoolRestartsOnNewSettings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newCameraPool(ctx)
	defer p.stopAll()
	cfg := camera.Config{Device: TestDevice, Width: 64, Height: 48, Fps: 5}

	first := p.acquire(cfg)
	p.release(TestDevice) // a channel changing settings releases, then acquires again
	cfg.Fps = 10
	second := p.acquire(cfg)
	assert.NotSame(t, first, second)
	assert.Equal(t, 10, second.Config().Fps)

	// A preview of the device while a channel holds it keeps the channel's settings.
	cfg.Fps = 2
	assert.Same(t, second, p.preview(cfg))
}
