package camera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config says which device to open and how.
type Config struct {
	// Device is a capture device: on Windows a DirectShow device path
	// ("@device_pnp_…", see Device.Path) or name ("Laptop Camera"), on Linux a path
	// (/dev/video0). "lavfi:testsrc" (any lavfi graph) is ffmpeg's test picture, for
	// trying things without a camera.
	Device string
	Width  int
	Height int
	Fps    int
	// Ffmpeg is the executable; empty means "ffmpeg" on PATH.
	Ffmpeg string
	// DetectWidth and DetectHeight, when smaller than the capture size, split the
	// capture in two: Latest frames are scaled down to this size (cheap to decode for
	// tracking) and LatestFull keeps the camera's own full-size JPEGs, copied as they
	// come. Then a real camera is asked for MJPEG, which high-resolution cameras send
	// at full rate (their raw modes run at a frame or two a second).
	DetectWidth  int
	DetectHeight int
	// SourceFps is the rate the camera sends a split capture at (from its mode list;
	// 0 assumes defaultSourceFps). Frames past Fps are dropped before they're decoded:
	// a 4K camera sends 30 a second, and decoding them all takes over a core.
	SourceFps float64
}

const defaultSourceFps = 30

// keepEvery is how many of the camera's frames make one at Fps.
func (c Config) keepEvery() int {
	src := c.SourceFps
	if src <= 0 {
		src = defaultSourceFps
	}
	if c.Fps <= 0 || src <= float64(c.Fps) {
		return 1
	}
	return max(1, int(src/float64(c.Fps)+0.5))
}

// Split is whether this capture keeps a full-size stream next to the scaled one.
func (c Config) Split() bool {
	return c.DetectWidth > 0 && c.DetectHeight > 0 && c.Width > 0 && c.Height > 0 &&
		(c.DetectWidth < c.Width || c.DetectHeight < c.Height)
}

func (c Config) ffmpeg() string {
	return ResolveFFmpeg(c.Ffmpeg)
}

// args is the ffmpeg command line for this camera: capture, then MJPEG on stdout.
// With Split, the full-size stream goes to fullURL as well.
func (c Config) args(fullURL string) []string {
	split := c.Split() && fullURL != ""
	lavfi := strings.HasPrefix(c.Device, "lavfi:")
	args := []string{"-loglevel", "error", "-nostdin"}
	switch {
	case lavfi:
		graph := strings.TrimPrefix(c.Device, "lavfi:")
		if c.Width > 0 && c.Height > 0 && !strings.Contains(graph, "size=") {
			sep := "="
			if strings.Contains(graph, "=") {
				sep = ":"
			}
			graph += fmt.Sprintf("%ssize=%dx%d", sep, c.Width, c.Height)
			if c.Fps > 0 {
				graph += fmt.Sprintf(":rate=%d", c.Fps)
			}
		}
		args = append(args, "-re", "-f", "lavfi", "-i", graph)
	default:
		if runtime.GOOS == "windows" {
			args = append(args, "-f", "dshow")
			if split {
				args = append(args, "-vcodec", "mjpeg")
			}
		} else {
			args = append(args, "-f", "v4l2")
			if split {
				args = append(args, "-input_format", "mjpeg")
			}
		}
		if c.Width > 0 && c.Height > 0 {
			args = append(args, "-video_size", fmt.Sprintf("%dx%d", c.Width, c.Height))
		}
		// No -framerate on the input: a camera only opens at a rate it offers (often
		// just 30), so it runs at its own rate and -r below drops frames to Fps.
		input := c.Device
		if runtime.GOOS == "windows" {
			input = "video=" + c.Device
		}
		if n := c.keepEvery(); split && n > 1 {
			// Drop the frames past Fps before decoding (both streams then run at Fps).
			args = append(args, "-bsf:v", fmt.Sprintf(`noise=drop=mod(n\,%d)`, n))
		}
		args = append(args, "-i", input)
	}
	if !split {
		if c.Fps > 0 {
			args = append(args, "-r", strconv.Itoa(c.Fps))
		}
		return append(args, "-an", "-f", "mjpeg", "-q:v", "4", "-")
	}
	// The scaled stream, on stdout.
	args = append(args, "-map", "0:v", "-vf", fmt.Sprintf("scale=%d:%d", c.DetectWidth, c.DetectHeight))
	if c.Fps > 0 {
		args = append(args, "-r", strconv.Itoa(c.Fps))
	}
	args = append(args, "-an", "-f", "mjpeg", "-q:v", "4", "-")
	// The full-size one: the camera's own JPEGs, untouched (mjpeg2jpeg adds the
	// Huffman tables some cameras leave out), or encoded for the test picture.
	args = append(args, "-map", "0:v", "-an")
	if lavfi {
		if c.Fps > 0 {
			args = append(args, "-r", strconv.Itoa(c.Fps))
		}
		args = append(args, "-c:v", "mjpeg", "-q:v", "2")
	} else {
		args = append(args, "-c:v", "copy", "-bsf:v", "mjpeg2jpeg")
	}
	return append(args, "-f", "mjpeg", fullURL)
}

// Frame is one JPEG from the camera.
type Frame struct {
	JPEG []byte
	At   time.Time
	Seq  uint64
}

// Status is how a camera is doing, for the UI.
type Status struct {
	Device  string    `json:"device"`
	Running bool      `json:"running"`
	Frames  uint64    `json:"frames"`
	LastAt  time.Time `json:"last_at,omitempty"`
	Fps     float64   `json:"fps"`
	Error   string    `json:"error,omitempty"`
}

// Camera keeps one capture running, restarting ffmpeg if it stops, and keeps the
// newest frame.
type Camera struct {
	cfg Config

	mu       sync.Mutex
	latest   Frame
	full     Frame   // the newest full-size frame, for a split capture
	srcFps   float64 // the camera's own rate at the capture size; -1 unknown
	running  bool
	err      string
	fpsCount int
	fpsSince time.Time
	fps      float64
	subs     map[chan Frame]struct{}
}

func New(cfg Config) *Camera {
	return &Camera{cfg: cfg, subs: map[chan Frame]struct{}{}}
}

// Config is what this camera was opened with.
func (c *Camera) Config() Config { return c.cfg }

const (
	restartMin = time.Second
	restartMax = 30 * time.Second
)

// Run captures until ctx ends.
func (c *Camera) Run(ctx context.Context) {
	wait := restartMin
	for ctx.Err() == nil {
		started := time.Now()
		err := c.capture(ctx)
		if ctx.Err() != nil {
			break
		}
		msg := "ffmpeg stopped"
		if err != nil {
			msg = err.Error()
		}
		c.setStatus(false, msg)
		slog.Warn("camera stopped; restarting", "device", c.cfg.Device, "error", msg, "after", wait)
		if time.Since(started) > restartMax {
			wait = restartMin // it ran a good while: start over gently
		}
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
		wait = min(wait*2, restartMax)
	}
	c.setStatus(false, "")
}

func (c *Camera) capture(ctx context.Context) error {
	// A split capture sends its full-size stream to a local port: ffmpeg connects,
	// and the frames are kept as they come.
	fullURL := ""
	if c.cfg.Split() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("full-size stream: %w", err)
		}
		defer ln.Close()
		fullURL = fmt.Sprintf("tcp://%s", ln.Addr())
		go c.receiveFull(ctx, ln)
	}
	cfg := c.cfg
	if cfg.Split() && cfg.SourceFps <= 0 {
		cfg.SourceFps = c.sourceFps(ctx)
	}
	cmd := exec.Command(cfg.ffmpeg(), cfg.args(fullURL)...)
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, n: 4096}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	// On cancel, stop ffmpeg and everything it started, and stop reading: a
	// grandchild still holding the pipe mustn't keep this capture alive.
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			killTree(cmd)
			_ = stdout.Close()
		case <-stopped:
		}
	}()
	c.setStatus(true, "")
	splitErr := SplitJPEGs(stdout, func(jpeg []byte) error {
		c.publish(append([]byte(nil), jpeg...))
		return nil
	})
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return errors.New("ffmpeg: " + lastLine(msg))
	}
	if splitErr != nil {
		return splitErr
	}
	return waitErr
}

// sourceFps asks the camera (once) how fast it sends MJPEG at the capture size; 0
// when it can't tell (the test picture, or not Windows), which assumes 30.
func (c *Camera) sourceFps(ctx context.Context) float64 {
	c.mu.Lock()
	known := c.srcFps
	c.mu.Unlock()
	if known != 0 || strings.HasPrefix(c.cfg.Device, "lavfi:") || runtime.GOOS != "windows" {
		return max(known, 0)
	}
	fps := -1.0 // asked: don't ask again
	if modes, err := ListModes(ctx, c.cfg.Ffmpeg, c.cfg.Device); err == nil {
		for _, m := range modes {
			if m.Format == "mjpeg" && m.Width == c.cfg.Width && m.Height == c.cfg.Height {
				fps = max(fps, m.Fps)
			}
		}
	}
	slog.Info("camera rate at its capture size", "device", c.cfg.Device, "size", fmt.Sprintf("%dx%d", c.cfg.Width, c.cfg.Height), "fps", fps)
	c.mu.Lock()
	c.srcFps = fps
	c.mu.Unlock()
	return max(fps, 0)
}

// receiveFull keeps the newest full-size frame from ffmpeg's connection, until the
// capture ends (ffmpeg exits, or ln is closed before it connects).
func (c *Camera) receiveFull(ctx context.Context, ln net.Listener) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = SplitJPEGs(conn, func(jpeg []byte) error {
		frame := append([]byte(nil), jpeg...)
		c.mu.Lock()
		c.full = Frame{JPEG: frame, At: time.Now(), Seq: c.full.Seq + 1}
		c.mu.Unlock()
		return nil
	})
}

func (c *Camera) publish(jpeg []byte) {
	now := time.Now()
	c.mu.Lock()
	c.latest = Frame{JPEG: jpeg, At: now, Seq: c.latest.Seq + 1}
	frame := c.latest
	c.fpsCount++
	if since := now.Sub(c.fpsSince); since >= 2*time.Second {
		c.fps = float64(c.fpsCount) / since.Seconds()
		c.fpsCount, c.fpsSince = 0, now
	}
	subs := make([]chan Frame, 0, len(c.subs))
	for ch := range c.subs {
		subs = append(subs, ch)
	}
	c.mu.Unlock()
	for _, ch := range subs {
		select {
		case <-ch: // drop the stale one for a slow viewer
		default:
		}
		select {
		case ch <- frame:
		default:
		}
	}
}

func (c *Camera) setStatus(running bool, err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running, c.err = running, err
	if running {
		c.fpsSince, c.fpsCount = time.Now(), 0
	}
}

// Latest is the newest frame, if there is one.
func (c *Camera) Latest() (Frame, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest, c.latest.Seq > 0
}

// LatestFull is the newest full-size frame of a split capture (see Config.Split).
func (c *Camera) LatestFull() (Frame, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.full, c.full.Seq > 0
}

// Subscribe sends each new frame (only the newest, if the reader falls behind).
func (c *Camera) Subscribe() (<-chan Frame, func()) {
	ch := make(chan Frame, 1)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.subs, ch)
		c.mu.Unlock()
	}
}

func (c *Camera) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{Device: c.cfg.Device, Running: c.running, Frames: c.latest.Seq, LastAt: c.latest.At, Fps: c.fps, Error: c.err}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// limitedWriter keeps the first n bytes and drops the rest.
type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		_, _ = l.w.Write(p[:k])
		l.n -= k
	}
	return len(p), nil
}
