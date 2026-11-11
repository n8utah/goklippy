package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Capturing a channel empty all the way round: a job turns it a step at a time and
// takes a picture at each, so the detector can compare each frame with the picture
// of the rotor standing where it is now (see vision/views.go).

// ViewCapturer takes a channel's pictures and keeps the finished set.
type ViewCapturer interface {
	ChannelFrameAfter(ctx context.Context, ch Channel, t time.Time) (image.Image, []byte, error)
	SetViews(ch Channel, jpegs [][]byte) error
}

const (
	defaultViewStep = 2.0 // degrees between pictures
	minViewStep     = 0.5
	maxViewStep     = 30.0
	viewSettle      = 600 * time.Millisecond // after each step, before the picture
)

// CaptureViews runs the round of pictures as a job. The channel must be clear: a
// piece on it would ride round into every picture.
func (m *Machine) CaptureViews(ctx context.Context, ch Channel, capturer ViewCapturer, step float64) error {
	if !isCameraChannel(ch) {
		return ErrNotCameraCh
	}
	if step == 0 {
		step = defaultViewStep
	}
	if step < minViewStep || step > maxViewStep {
		return fmt.Errorf("%w: take a picture every %g to %g degrees", ErrInvalidSettings, minViewStep, maxViewStep)
	}
	n := int(math.Round(360 / step))
	step = 360 / float64(n) // so the round ends where it began
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		if !m.health.Connected {
			return ErrNoBoard
		}
		name := fmt.Sprintf("Capture %s empty all the way round", strings.ToUpper(string(ch)))
		return m.startJob(name, func(ctx context.Context, report reportFunc) error {
			rpm := findRpm(m.settings(), ch)
			jpegs := make([][]byte, 0, n)
			for i := 0; i < n; i++ {
				report(i+1, n, fmt.Sprintf("Picture at %.1f°", float64(i)*step))
				if err := waitFor(ctx, viewSettle); err != nil {
					return err
				}
				fctx, cancel := context.WithTimeout(ctx, findFrameTimeout)
				_, jpg, err := capturer.ChannelFrameAfter(fctx, ch, time.Now())
				cancel()
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return fmt.Errorf("picture at %.1f°: %w", float64(i)*step, err)
				}
				jpegs = append(jpegs, jpg)
				if err := m.turnAndWait(ctx, ch, step, rpm); err != nil {
					return fmt.Errorf("turn %s: %w", ch, err)
				}
			}
			return capturer.SetViews(ch, jpegs)
		}, nil)
	}})
}

func isCameraChannel(ch Channel) bool {
	for _, c := range CameraChannels {
		if c == ch {
			return true
		}
	}
	return false
}

// ChannelFrameAfter is the channel's camera's frame taken after t.
func (p *Perception) ChannelFrameAfter(ctx context.Context, ch Channel, t time.Time) (image.Image, []byte, error) {
	device := p.settings().Cameras.Devices[string(ch)]
	if device == "" {
		return nil, nil, ErrNoCamera
	}
	return p.FrameAfter(ctx, device, t)
}

// SetViews gives the channel's detector its all-the-way-round pictures and keeps them
// on disk for the next start, replacing a single empty-channel picture.
func (p *Perception) SetViews(ch Channel, jpegs [][]byte) error {
	imgs, err := decodeAll(jpegs)
	if err != nil {
		return err
	}
	p.mu.Lock()
	pc := p.chans[ch]
	if pc == nil {
		p.mu.Unlock()
		return ErrNoCamera
	}
	err = pc.detector.SetViews(imgs)
	pc.tracker.Reset()
	p.mu.Unlock()
	if err != nil {
		return err
	}
	dir := p.viewsDir(ch)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, jpg := range jpegs {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("view-%03d.jpg", i)), jpg, 0o644); err != nil {
			return err
		}
	}
	_ = os.Remove(p.backgroundPath(ch)) // the views replace it
	return nil
}

func (p *Perception) viewsDir(ch Channel) string {
	return filepath.Join(p.dataDir, "backgrounds", string(ch)+"-views")
}

// loadViews reads a channel's saved views, in order; none is not an error.
func (p *Perception) loadViews(ch Channel) ([]image.Image, error) {
	paths, err := filepath.Glob(filepath.Join(p.viewsDir(ch), "view-*.jpg"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	sort.Strings(paths)
	jpegs := make([][]byte, len(paths))
	for i, path := range paths {
		if jpegs[i], err = os.ReadFile(path); err != nil {
			return nil, err
		}
	}
	return decodeAll(jpegs)
}

func decodeAll(jpegs [][]byte) ([]image.Image, error) {
	if len(jpegs) == 0 {
		return nil, errors.New("no pictures")
	}
	imgs := make([]image.Image, len(jpegs))
	for i, b := range jpegs {
		img, err := jpeg.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("picture %d: %w", i, err)
		}
		imgs[i] = img
	}
	return imgs, nil
}
