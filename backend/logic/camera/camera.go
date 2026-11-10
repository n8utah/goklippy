package camera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config says which device to open and how.
type Config struct {
	// Device is a capture device name: on Windows the DirectShow name ("Laptop
	// Camera"), on Linux a path (/dev/video0). "lavfi:testsrc" (any lavfi graph) is
	// ffmpeg's test picture, for trying things without a camera.
	Device string
	Width  int
	Height int
	Fps    int
	// Ffmpeg is the executable; empty means "ffmpeg" on PATH.
	Ffmpeg string
}

func (c Config) ffmpeg() string {
	if c.Ffmpeg != "" {
		return c.Ffmpeg
	}
	return "ffmpeg"
}

// args is the ffmpeg command line for this camera: capture, then MJPEG on stdout.
func (c Config) args() []string {
	args := []string{"-loglevel", "error", "-nostdin"}
	switch {
	case strings.HasPrefix(c.Device, "lavfi:"):
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
		} else {
			args = append(args, "-f", "v4l2")
		}
		if c.Width > 0 && c.Height > 0 {
			args = append(args, "-video_size", fmt.Sprintf("%dx%d", c.Width, c.Height))
		}
		if c.Fps > 0 {
			args = append(args, "-framerate", strconv.Itoa(c.Fps))
		}
		input := c.Device
		if runtime.GOOS == "windows" {
			input = "video=" + c.Device
		}
		args = append(args, "-i", input)
	}
	if c.Fps > 0 {
		args = append(args, "-r", strconv.Itoa(c.Fps))
	}
	return append(args, "-an", "-f", "mjpeg", "-q:v", "4", "-")
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
	cmd := exec.Command(c.cfg.ffmpeg(), c.cfg.args()...)
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

// ListDevices asks ffmpeg for the video capture devices (DirectShow on Windows).
func ListDevices(ctx context.Context, ffmpeg string) ([]string, error) {
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if runtime.GOOS != "windows" {
		return nil, errors.New("listing devices is only done on Windows; give a /dev/videoN path")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	list := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, ffmpeg, append(args, "-list_devices", "true", "-f", "dshow", "-i", "dummy")...)
		hideWindow(cmd)
		out, err := cmd.CombinedOutput() // it always "fails": dummy isn't a device
		if len(out) == 0 && err != nil {
			return "", fmt.Errorf("run ffmpeg: %w", err)
		}
		return string(out), nil
	}
	out, err := list("-hide_banner")
	if err == nil && strings.Contains(out, "Unrecognized option") {
		out, err = list() // an old ffmpeg without -hide_banner
	}
	if err != nil {
		return nil, err
	}
	return ParseDshowDevices(out), nil
}

var (
	// New ffmpeg: `[in#0 @ 000001] "Laptop Camera" (video)`.
	dshowDevice = regexp.MustCompile(`"([^"]+)"\s+\(video\)`)
	// Old ffmpeg lists "DirectShow video devices" then names, then audio devices.
	dshowHeading = regexp.MustCompile(`DirectShow (video|audio) devices`)
	dshowName    = regexp.MustCompile(`\]\s+"([^"]+)"\s*$`)
)

// ParseDshowDevices pulls video device names out of ffmpeg's -list_devices output.
func ParseDshowDevices(out string) []string {
	var devices []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			devices = append(devices, name)
		}
	}
	section := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.Contains(line, "Alternative name") {
			continue
		}
		if m := dshowDevice.FindStringSubmatch(line); m != nil {
			add(m[1])
			continue
		}
		if m := dshowHeading.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		if section == "video" {
			if m := dshowName.FindStringSubmatch(line); m != nil {
				add(m[1])
			}
		}
	}
	return devices
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
