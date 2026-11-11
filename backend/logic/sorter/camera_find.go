package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Finding which camera is where: every camera is watched while each channel turns a
// little and back. The camera whose picture changes most when a channel turns is over
// that channel. The result is a proposal; the Cameras page applies it.

// FrameSource gives decoded frames from any camera by device id. Perception is one.
type FrameSource interface {
	// FrameAfter waits for a frame taken after t and returns it decoded, with its JPEG.
	FrameAfter(ctx context.Context, device string, t time.Time) (image.Image, []byte, error)
}

// CameraFindRequest is which cameras to watch and how far to turn each channel.
type CameraFindRequest struct {
	Devices []string `json:"devices"`
	Degrees float64  `json:"degrees"` // rotor degrees per channel; 0 uses defaultFindDegrees
}

// CameraFindResult is what a run found.
type CameraFindResult struct {
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	Devices  []string  `json:"devices"`
	// CenterShare is how much of each picture's width and height, in the middle, was
	// compared.
	CenterShare float64 `json:"center_share"`
	// Noise is each camera's change between two frames with nothing moving.
	Noise map[string]float64 `json:"noise"`
	// Scores is, per channel turned, each camera's share of the picture that changed.
	Scores map[Channel]map[string]float64 `json:"scores"`
	// Proposed is the camera for each of C2, C3 and C4 (missing if none saw it).
	Proposed map[Channel]string `json:"proposed"`
	Notes    []string           `json:"notes"`
	Error    string             `json:"error,omitempty"`

	images map[Channel]findImages // before/after of each channel's best camera
}

type findImages struct {
	device        string
	before, after []byte
}

var (
	ErrNotFinding  = errors.New("there's no camera search result yet")
	ErrNoImage     = errors.New("no picture for that channel")
	ErrNoMotion    = errors.New("no camera saw any channel move: are the motors powered (the RAMPS 12 V 5 A input)? Is anything that turns in view?")
	errNoFindInput = errors.New("pick at least one camera to watch")
)

const (
	defaultFindDegrees = 15
	findSettle         = 800 * time.Millisecond // after a move, before the picture
	findFrameTimeout   = 8 * time.Second
	// A pixel has changed when |ΔR|+|ΔG|+|ΔB| reaches this (0–765).
	findPixelThreshold = 60
	findSampleStep     = 4
	// A camera saw a channel move when its change beats both of these.
	findMinScore    = 0.003 // 0.3% of the picture
	findNoiseFactor = 3     // times its own noise
	// Two cameras this close for one channel are worth a look.
	findAmbiguousRatio = 1.5
)

// CameraFinder keeps the last search's result for the page.
type CameraFinder struct {
	mu   sync.Mutex
	last *CameraFindResult
}

func NewCameraFinder() *CameraFinder { return &CameraFinder{} }

// Last is the last result, without its pictures.
func (f *CameraFinder) Last() (CameraFindResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == nil {
		return CameraFindResult{}, ErrNotFinding
	}
	r := *f.last
	r.images = nil
	return r, nil
}

// Image is the before or after picture of a channel's best camera.
func (f *CameraFinder) Image(ch Channel, after bool) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == nil {
		return nil, ErrNotFinding
	}
	img, ok := f.last.images[ch]
	if !ok {
		return nil, ErrNoImage
	}
	if after {
		return img.after, nil
	}
	return img.before, nil
}

func (f *CameraFinder) set(r *CameraFindResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last = r
}

// FindCameras runs the search as a job: nothing else may move meanwhile, and Cancel
// stops it.
func (m *Machine) FindCameras(ctx context.Context, src FrameSource, finder *CameraFinder, req CameraFindRequest) error {
	if len(req.Devices) == 0 {
		return fmt.Errorf("%w: %v", ErrInvalidSettings, errNoFindInput)
	}
	if req.Degrees == 0 {
		req.Degrees = defaultFindDegrees
	}
	if req.Degrees < 0 || req.Degrees > 90 {
		return fmt.Errorf("%w: turn each channel between 0 and 90 degrees", ErrInvalidSettings)
	}
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		if !m.health.Connected {
			return ErrNoBoard
		}
		return m.startJob("Find which camera is where", func(ctx context.Context, report reportFunc) error {
			r := &CameraFindResult{Started: time.Now().UTC(), Devices: req.Devices, CenterShare: FindCenterShare}
			err := m.findCameras(ctx, src, req, r, report)
			r.Finished = time.Now().UTC()
			if err != nil {
				r.Error = err.Error()
			}
			finder.set(r)
			return err
		}, nil)
	}})
}

func (m *Machine) findCameras(ctx context.Context, src FrameSource, req CameraFindRequest, r *CameraFindResult, report reportFunc) error {
	channels := []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4}
	total := 1 + len(channels)
	r.Noise = map[string]float64{}
	r.Scores = map[Channel]map[string]float64{}
	r.Proposed = map[Channel]string{}
	r.images = map[Channel]findImages{}

	report(1, total, "Starting the cameras and measuring their noise")
	ref, err := m.grabAll(ctx, src, req.Devices, time.Time{})
	if err != nil {
		return err
	}
	if err := waitFor(ctx, findSettle); err != nil {
		return err
	}
	again, err := m.grabAll(ctx, src, req.Devices, time.Now())
	if err != nil {
		return err
	}
	for _, d := range req.Devices {
		r.Noise[d] = changedShare(ref[d].img, again[d].img)
	}

	settings := m.settings()
	for i, ch := range channels {
		report(2+i, total, fmt.Sprintf("Turning %s %.0f° and back", strings.ToUpper(string(ch)), req.Degrees))
		before, err := m.grabAll(ctx, src, req.Devices, time.Now())
		if err != nil {
			return err
		}
		rpm := findRpm(settings, ch)
		if err := m.turnAndWait(ctx, ch, req.Degrees, rpm); err != nil {
			return fmt.Errorf("turn %s: %w", ch, err)
		}
		if err := waitFor(ctx, findSettle); err != nil {
			return err
		}
		after, grabErr := m.grabAll(ctx, src, req.Devices, time.Now())
		// Always turn back, so the machine is left as it was.
		if err := m.turnAndWait(ctx, ch, -req.Degrees, rpm); err != nil {
			return fmt.Errorf("turn %s back: %w", ch, err)
		}
		if grabErr != nil {
			return grabErr
		}
		r.Scores[ch] = map[string]float64{}
		best, bestScore := "", -1.0
		for _, d := range req.Devices {
			s := changedShare(before[d].img, after[d].img)
			r.Scores[ch][d] = s
			if s > bestScore {
				best, bestScore = d, s
			}
		}
		r.images[ch] = findImages{device: best, before: before[best].jpeg, after: after[best].jpeg}
	}
	return r.decide()
}

// decide turns the scores into a proposal and notes.
func (r *CameraFindResult) decide() error {
	saw := func(d string, s float64) bool {
		return s >= findMinScore && s >= findNoiseFactor*r.Noise[d]
	}
	type cand struct {
		ch    Channel
		dev   string
		score float64
	}
	var cands []cand
	anyMotion := false
	for ch, scores := range r.Scores {
		for d, s := range scores {
			if saw(d, s) {
				anyMotion = true
				if ch != ChannelC1 {
					cands = append(cands, cand{ch, d, s})
				}
			}
		}
	}
	if !anyMotion {
		return ErrNoMotion
	}
	// The strongest pairs first; a camera watches one channel, a channel has one camera.
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	used := map[string]bool{}
	for _, c := range cands {
		if _, taken := r.Proposed[c.ch]; taken || used[c.dev] {
			continue
		}
		r.Proposed[c.ch] = c.dev
		used[c.dev] = true
	}
	for _, ch := range CameraChannels {
		dev, ok := r.Proposed[ch]
		if !ok {
			r.Notes = append(r.Notes, fmt.Sprintf("No camera saw %s move: its camera may be unplugged or not picked, or nothing on %s shows a turn (put a piece or a strip of tape on it).", strings.ToUpper(string(ch)), strings.ToUpper(string(ch))))
			continue
		}
		for d, s := range r.Scores[ch] {
			if d != dev && saw(d, s) && r.Scores[ch][dev] < findAmbiguousRatio*s {
				r.Notes = append(r.Notes, fmt.Sprintf("Two cameras saw %s move about as much; check the pictures before using this.", strings.ToUpper(string(ch))))
				break
			}
		}
	}
	// C1 has no camera, so a camera that sees it move is worth a word.
	c1Unused, c1Used := false, false
	for d, s := range r.Scores[ChannelC1] {
		if saw(d, s) {
			if used[d] {
				c1Used = true
			} else {
				c1Unused = true
			}
		}
	}
	switch {
	case c1Unused:
		r.Notes = append(r.Notes, "A camera saw C1 move but no other channel. C1 has no camera; check where that camera is pointed.")
	case c1Used:
		r.Notes = append(r.Notes, "Turning C1 also changed a camera's picture (pieces moving onto C2?). That's fine if it's C2's camera.")
	}
	sort.Strings(r.Notes)
	return nil
}

func findRpm(s MachineSettings, ch Channel) float64 {
	if ch == ChannelC4 {
		if s.IndexRpm > 0 {
			return s.IndexRpm
		}
	} else if rpm := s.FeederRpm[string(ch)]; rpm > 0 {
		return rpm
	}
	return 5
}

// turnAndWait turns a channel and waits for the move to end.
func (m *Machine) turnAndWait(ctx context.Context, ch Channel, deg, rpm float64) error {
	done, err := m.hw.MoveChannel(ch, deg, rpm)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

type grabbed struct {
	img  image.Image
	jpeg []byte
}

func (m *Machine) grabAll(ctx context.Context, src FrameSource, devices []string, after time.Time) (map[string]grabbed, error) {
	out := make(map[string]grabbed, len(devices))
	for _, d := range devices {
		fctx, cancel := context.WithTimeout(ctx, findFrameTimeout)
		img, jpg, err := src.FrameAfter(fctx, d, after)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("camera %s: %w", d, err)
		}
		out[d] = grabbed{img: img, jpeg: jpg}
	}
	return out, nil
}

// findCenter is the part of each picture compared: the middle FindCenterShare of its
// width and height. A camera's edges can show a neighboring channel, which would make
// it look like it watches that one too; its own channel fills the middle.
const FindCenterShare = 0.5

// changedShare is the share of sampled pixels in the middle of the frame that changed
// between two frames. Frames of different sizes count as all changed.
func changedShare(a, b image.Image) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return 1
	}
	margin := (1 - FindCenterShare) / 2
	x0, x1 := int(float64(ab.Dx())*margin), int(float64(ab.Dx())*(1-margin))
	y0, y1 := int(float64(ab.Dy())*margin), int(float64(ab.Dy())*(1-margin))
	changed, total := 0, 0
	for y := y0; y < y1; y += findSampleStep {
		for x := x0; x < x1; x += findSampleStep {
			r1, g1, b1, _ := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			r2, g2, b2, _ := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			d := absDiff(r1, r2) + absDiff(g1, g2) + absDiff(b1, b2)
			if d>>8 >= findPixelThreshold {
				changed++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return math.Round(float64(changed)/float64(total)*1e5) / 1e5
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// FrameAfter is a camera's frame taken after t, for the camera search. It keeps the
// camera's preview running while it waits.
func (p *Perception) FrameAfter(ctx context.Context, device string, t time.Time) (image.Image, []byte, error) {
	for {
		jpg, at, _, err := p.Preview(device)
		if errors.Is(err, ErrUnknownDevice) {
			return nil, nil, err
		}
		if err == nil && at.After(t) {
			img, err := jpeg.Decode(bytes.NewReader(jpg))
			if err != nil {
				return nil, nil, err
			}
			return img, jpg, nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return nil, nil, fmt.Errorf("no picture: %w", err)
			}
			return nil, nil, errors.New("no new picture in time")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
