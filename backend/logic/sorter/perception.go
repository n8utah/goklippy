package sorter

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"goklippy/backend/logic/camera"
	"goklippy/backend/logic/vision"
)

// CameraChannels are the channels with a camera over them.
var CameraChannels = []Channel{ChannelC2, ChannelC3, ChannelC4}

var (
	ErrNoCamera    = errors.New("no camera is assigned to that channel")
	ErrNoFrame     = errors.New("the camera hasn't sent a frame yet")
	ErrNotCameraCh = errors.New("only c2, c3 and c4 have cameras")
)

// CameraFeed is what the camera routes show: the newest JPEG per channel and what
// perception made of it.
type CameraFeed interface {
	JPEG(ch Channel) ([]byte, time.Time, bool)
	Observation(ch Channel) vision.Observation
}

// CameraStatus is one channel's camera for the UI.
type CameraStatus struct {
	Device        string        `json:"device"`
	Camera        camera.Status `json:"camera"`
	HasBackground bool          `json:"has_background"`
	// Views is how many empty-channel pictures it has all the way round (0: one
	// picture, or none), and MatchedView the one the last frame matched.
	Views       int    `json:"views"`
	MatchedView int    `json:"matched_view"`
	ZonesSet    bool   `json:"zones_set"`
	FrameW      int    `json:"frame_w,omitempty"`
	FrameH      int    `json:"frame_h,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Perception runs a camera per channel that has one and turns its frames into
// observations: the background detector, the channel's zones and the tracker. It is
// the machine's vision when real cameras are fitted.
type Perception struct {
	settings func() MachineSettings
	dataDir  string
	pool     *cameraPool

	mu    sync.Mutex
	chans map[Channel]*perceptionChannel
	known map[string]bool // devices the last listing found, which may be previewed
}

type perceptionChannel struct {
	cfg      camera.Config
	cam      *camera.Camera     // from the pool, shared with previews
	cancel   context.CancelFunc // stops this channel's frame processing
	detector *vision.BackgroundDetector
	tracker  *vision.Tracker

	obs     vision.Observation
	img     image.Image
	imgAt   time.Time
	message string
}

func NewPerception(settings func() MachineSettings, dataDir string) *Perception {
	return &Perception{
		settings: settings, dataDir: dataDir,
		pool:  newCameraPool(context.Background()),
		chans: map[Channel]*perceptionChannel{},
		known: map[string]bool{},
	}
}

const reconcileEvery = time.Second

// Run keeps the cameras matching the settings until ctx ends.
func (p *Perception) Run(ctx context.Context) {
	ticker := time.NewTicker(reconcileEvery)
	defer ticker.Stop()
	for {
		p.reconcile(ctx)
		p.pool.sweep(time.Now())
		select {
		case <-ctx.Done():
			p.mu.Lock()
			for _, c := range p.chans {
				c.cancel()
			}
			p.mu.Unlock()
			p.pool.stopAll()
			return
		case <-ticker.C:
		}
	}
}

// cameraConfig is how every camera is opened, from the settings.
func cameraConfig(cs CameraSettings, device string) camera.Config {
	return camera.Config{Device: device, Width: cs.Width, Height: cs.Height, Fps: cs.Fps, Ffmpeg: cs.Ffmpeg}
}

// TestDevice is ffmpeg's test picture, which can always be previewed.
const TestDevice = "lavfi:testsrc"

var ErrUnknownDevice = errors.New("that camera isn't in the last device list; look for cameras again")

// NoteDevices records what a device listing found: those may be previewed.
func (p *Perception) NoteDevices(ids []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.known = map[string]bool{}
	for _, id := range ids {
		p.known[id] = true
	}
}

// MayOpen is whether a device may be handed to ffmpeg from a request: one the last
// listing found, one a channel is set to, or the test picture.
func (p *Perception) MayOpen(device string) bool {
	if device == "" {
		return false
	}
	p.mu.Lock()
	allowed := p.known[device] || device == TestDevice
	p.mu.Unlock()
	for _, d := range p.settings().Cameras.Devices {
		allowed = allowed || d == device
	}
	return allowed
}

// Preview is a camera's newest frame for the Cameras page, whether or not a channel
// uses it. The first call starts its capture; it keeps running while it's watched.
// Only listed devices, assigned ones and the test picture can be previewed: the
// device goes to ffmpeg.
func (p *Perception) Preview(device string) ([]byte, time.Time, camera.Status, error) {
	if !p.MayOpen(device) {
		return nil, time.Time{}, camera.Status{}, ErrUnknownDevice
	}
	cam := p.pool.preview(cameraConfig(p.settings().Cameras, device))
	f, ok := cam.Latest()
	if !ok {
		return nil, time.Time{}, cam.Status(), ErrNoFrame
	}
	return f.JPEG, f.At, cam.Status(), nil
}

// reconcile starts, stops or restarts cameras whose settings changed.
func (p *Perception) reconcile(ctx context.Context) {
	cs := p.settings().Cameras
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ch := range CameraChannels {
		want := cameraConfig(cs, cs.Devices[string(ch)])
		cur := p.chans[ch]
		if cur != nil && cur.cfg == want {
			// Same frame size, so the step (which the background was sampled at) stays.
			cur.detector.Threshold, cur.detector.MinArea, _ = detectorParams(cs)
			continue
		}
		if cur != nil {
			cur.cancel()
			p.pool.release(cur.cfg.Device)
			delete(p.chans, ch)
		}
		if want.Device == "" {
			continue
		}
		cctx, cancel := context.WithCancel(ctx)
		pc := &perceptionChannel{cfg: want, cam: p.pool.acquire(want), cancel: cancel, detector: vision.NewBackgroundDetector(), tracker: vision.NewTracker()}
		pc.detector.Threshold, pc.detector.MinArea, pc.detector.Step = detectorParams(cs)
		if views, spacing, err := p.loadViews(ch); err == nil && len(views) > 0 {
			if err := pc.detector.SetViews(views, spacing); err != nil {
				slog.Warn("saved empty-channel pictures not used", "channel", ch, "error", err)
			}
		} else if bg, err := p.loadBackground(ch); err == nil {
			pc.detector.SetBackground(bg)
		}
		p.chans[ch] = pc
		go p.process(cctx, ch, pc)
		slog.Info("camera started", "channel", ch, "device", want.Device)
	}
}

// The detector's defaults are for a 640×480 frame.
const detectorBaseW, detectorBaseH = 640, 480

// detectorParams are the detector's settings for the cameras' frame size. Unset ones
// scale from the 640×480 defaults: the smallest piece by pixel count, and the sampling
// step by width, so a 1280×720 frame costs about what a 640×480 one does.
func detectorParams(cs CameraSettings) (threshold, minArea, step int) {
	d := vision.NewBackgroundDetector()
	threshold, minArea, step = d.Threshold, d.MinArea, d.Step
	if cs.Width > 0 && cs.Height > 0 {
		scale := float64(cs.Width*cs.Height) / (detectorBaseW * detectorBaseH)
		minArea = max(1, int(float64(d.MinArea)*scale+0.5))
		step = max(d.Step, d.Step*cs.Width/detectorBaseW)
	}
	if cs.Threshold > 0 {
		threshold = cs.Threshold
	}
	if cs.MinArea > 0 {
		minArea = cs.MinArea
	}
	return threshold, minArea, step
}

// mergeGap joins detections this close (pixels): one piece in two colors.
const mergeGap = 6

// process turns each new frame into an observation.
func (p *Perception) process(ctx context.Context, ch Channel, pc *perceptionChannel) {
	frames, stop := pc.cam.Subscribe()
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-frames:
			img, err := jpeg.Decode(bytes.NewReader(f.JPEG))
			if err != nil {
				continue
			}
			obs, msg := p.observe(ch, pc, img, f.At)
			p.mu.Lock()
			pc.img, pc.imgAt, pc.obs, pc.message = img, f.At, obs, msg
			p.mu.Unlock()
		}
	}
}

// observe runs the detector over one frame. Without zones or a background picture
// there's no observation (Valid false), so the stations wait.
func (p *Perception) observe(ch Channel, pc *perceptionChannel, img image.Image, at time.Time) (vision.Observation, string) {
	zones, ok := p.settings().Cameras.Zones[string(ch)]
	if !ok || !zones.Valid() {
		return vision.Observation{}, "Draw this channel's zones on the Cameras page."
	}
	b := img.Bounds()
	zones = zones.ScaledTo(b.Dx(), b.Dy())
	p.mu.Lock()
	boxes, err := pc.detector.Detect(img, zones)
	p.mu.Unlock()
	if err != nil {
		return vision.Observation{}, err.Error()
	}
	obs := vision.Observe(zones, boxes, mergeGap, at)
	colors := make([][3]float64, len(obs.Pieces))
	for i, piece := range obs.Pieces {
		colors[i] = vision.MeanColor(img, piece.Box, 2)
	}
	p.mu.Lock() // CaptureBackground resets the tracker
	pc.tracker.Update(zones, &obs, colors)
	p.mu.Unlock()
	return obs, ""
}

// Observation is the newest observation of a channel.
func (p *Perception) Observation(ch Channel) vision.Observation {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc := p.chans[ch]; pc != nil {
		return pc.obs
	}
	return vision.Observation{}
}

// Frame is the newest decoded frame.
func (p *Perception) Frame(ch Channel) (image.Image, time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc := p.chans[ch]; pc != nil && pc.img != nil {
		return pc.img, pc.imgAt, true
	}
	return nil, time.Time{}, false
}

// JPEG is the newest frame as the camera sent it.
func (p *Perception) JPEG(ch Channel) ([]byte, time.Time, bool) {
	p.mu.Lock()
	pc := p.chans[ch]
	p.mu.Unlock()
	if pc == nil {
		return nil, time.Time{}, false
	}
	f, ok := pc.cam.Latest()
	return f.JPEG, f.At, ok
}

// CaptureBackground takes the newest frame as the empty channel, and keeps it for
// the next start.
func (p *Perception) CaptureBackground(ch Channel) error {
	p.mu.Lock()
	pc := p.chans[ch]
	p.mu.Unlock()
	if pc == nil {
		return ErrNoCamera
	}
	frame, ok := pc.cam.Latest()
	if !ok {
		return ErrNoFrame
	}
	img, err := jpeg.Decode(bytes.NewReader(frame.JPEG))
	if err != nil {
		return err
	}
	p.mu.Lock()
	pc.detector.SetBackground(img)
	pc.tracker.Reset()
	p.mu.Unlock()
	path := p.backgroundPath(ch)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(p.viewsDir(ch)) // one picture replaces the round
	return os.WriteFile(path, frame.JPEG, 0o644)
}

func (p *Perception) backgroundPath(ch Channel) string {
	return filepath.Join(p.dataDir, "backgrounds", string(ch)+".jpg")
}

func (p *Perception) loadBackground(ch Channel) (image.Image, error) {
	data, err := os.ReadFile(p.backgroundPath(ch))
	if err != nil {
		return nil, err
	}
	return jpeg.Decode(bytes.NewReader(data))
}

// Statuses describes each camera channel.
func (p *Perception) Statuses() map[Channel]CameraStatus {
	cs := p.settings().Cameras
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[Channel]CameraStatus{}
	for _, ch := range CameraChannels {
		z, ok := cs.Zones[string(ch)]
		st := CameraStatus{Device: cs.Devices[string(ch)], ZonesSet: ok && z.Valid()}
		if pc := p.chans[ch]; pc != nil {
			st.Camera = pc.cam.Status()
			st.HasBackground = pc.detector.HasBackground()
			st.Views, st.MatchedView = pc.detector.Views(), pc.detector.LastView()
			st.Message = pc.message
			if pc.img != nil {
				st.FrameW, st.FrameH = pc.img.Bounds().Dx(), pc.img.Bounds().Dy()
			}
		}
		out[ch] = st
	}
	return out
}

// SimFeed shows the simulated world as camera pictures.
type SimFeed struct {
	World *SimWorld
}

func (f SimFeed) JPEG(ch Channel) ([]byte, time.Time, bool) {
	img, at, ok := f.World.Frame(ch)
	if !ok {
		return nil, time.Time{}, false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil, time.Time{}, false
	}
	return buf.Bytes(), at, true
}

func (f SimFeed) Observation(ch Channel) vision.Observation {
	return f.World.Observation(ch)
}
