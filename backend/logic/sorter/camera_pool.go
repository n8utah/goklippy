package sorter

import (
	"context"
	"sync"
	"time"

	"goklippy/backend/logic/camera"
)

// A capture device can be opened by one ffmpeg at a time, so every use of a camera
// goes through one pool: a channel using it and anyone previewing it on the Cameras
// page share the same capture.

const (
	// previewHold keeps a preview's capture running this long after it was last
	// watched, so a page that polls doesn't restart ffmpeg each time.
	previewHold = 15 * time.Second
	// replaceWait is how long a new capture waits for the old one on the same device
	// to let go before opening it.
	replaceWait = 3 * time.Second
)

type cameraPool struct {
	ctx  context.Context
	mu   sync.Mutex
	cams map[string]*pooledCamera // by device
}

type pooledCamera struct {
	cfg          camera.Config
	cam          *camera.Camera
	cancel       context.CancelFunc
	done         chan struct{} // closed when its capture has stopped
	users        int           // channels using it
	previewUntil time.Time
}

func newCameraPool(ctx context.Context) *cameraPool {
	return &cameraPool{ctx: ctx, cams: map[string]*pooledCamera{}}
}

// get returns the running capture for cfg, starting it (or restarting it with new
// settings) as needed. Call with mu held.
func (p *cameraPool) get(cfg camera.Config) *pooledCamera {
	cur := p.cams[cfg.Device]
	if cur != nil && cur.cfg == cfg {
		return cur
	}
	var wait chan struct{}
	if cur != nil {
		cur.cancel() // same device, new settings: users move to the new capture
		wait = cur.done
	}
	ctx, cancel := context.WithCancel(p.ctx)
	pc := &pooledCamera{cfg: cfg, cam: camera.New(cfg), cancel: cancel, done: make(chan struct{})}
	if cur != nil {
		pc.users, pc.previewUntil = cur.users, cur.previewUntil
	}
	p.cams[cfg.Device] = pc
	go func() {
		defer close(pc.done)
		if wait != nil {
			select {
			case <-wait:
			case <-time.After(replaceWait):
			case <-ctx.Done():
				return
			}
		}
		pc.cam.Run(ctx)
	}()
	return pc
}

// acquire is a channel starting to use a camera.
func (p *cameraPool) acquire(cfg camera.Config) *camera.Camera {
	p.mu.Lock()
	defer p.mu.Unlock()
	pc := p.get(cfg)
	pc.users++
	return pc.cam
}

// release is a channel done with a camera; it stops unless previewed.
func (p *cameraPool) release(device string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc := p.cams[device]; pc != nil && pc.users > 0 {
		pc.users--
	}
	p.sweepLocked(time.Now())
}

// preview is the Cameras page watching a camera; it keeps running for previewHold.
func (p *cameraPool) preview(cfg camera.Config) *camera.Camera {
	p.mu.Lock()
	defer p.mu.Unlock()
	pc := p.cams[cfg.Device]
	if pc == nil || pc.users == 0 {
		pc = p.get(cfg) // a channel's capture keeps its own settings
	}
	pc.previewUntil = time.Now().Add(previewHold)
	return pc.cam
}

// running is the capture of a device, if one is running.
func (p *cameraPool) running(device string) *camera.Camera {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc := p.cams[device]; pc != nil {
		return pc.cam
	}
	return nil
}

// sweep stops captures nobody uses or watches any more.
func (p *cameraPool) sweep(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked(now)
}

func (p *cameraPool) sweepLocked(now time.Time) {
	for device, pc := range p.cams {
		if pc.users == 0 && now.After(pc.previewUntil) {
			pc.cancel()
			delete(p.cams, device)
		}
	}
}

func (p *cameraPool) stopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for device, pc := range p.cams {
		pc.cancel()
		delete(p.cams, device)
	}
}
