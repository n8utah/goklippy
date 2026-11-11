package vision

import (
	"errors"
	"image"
	"image/color"
)

// Detector finds pieces in a frame.
type Detector interface {
	Detect(frame image.Image, z ChannelZones) ([]Box, error)
}

var ErrNoBackground = errors.New("no empty-channel picture yet; capture one with the channel clear")

// BackgroundDetector finds pieces as what differs from a picture of the empty
// channel. It looks only at the channel's band, works on a grid of every Step-th
// pixel, and slowly blends unchanged pixels into the background so slow lighting
// drift doesn't build up. It suits a plain channel under steady light; a textured
// or patterned rotor would show its pattern turning as pieces.
type BackgroundDetector struct {
	// Threshold is the summed |ΔR|+|ΔG|+|ΔB| (0..765) at which a pixel counts as
	// changed.
	Threshold int
	// MinArea is the smallest piece, in full-resolution pixels.
	MinArea int
	// Step samples every Step-th pixel in x and y (1 = every pixel).
	Step int
	// Learn blends this share of each unchanged pixel into the background per
	// frame (0 = never).
	Learn float64

	bg     []float64 // RGB per sampled pixel
	bgW    int
	bgH    int
	mask   []bool // sampled pixels on the channel
	maskOf ChannelZones

	// views are empty-channel pictures all the way round (see views.go); when set
	// they're used instead of bg.
	views    [][]uint8
	lastView int
}

// NewBackgroundDetector has defaults for a 640×480 camera.
func NewBackgroundDetector() *BackgroundDetector {
	return &BackgroundDetector{Threshold: 90, MinArea: 120, Step: 2, Learn: 0.01, lastView: -1}
}

// HasBackground reports whether an empty-channel picture (or a set of them all the
// way round) has been captured.
func (d *BackgroundDetector) HasBackground() bool {
	return d.bg != nil || len(d.views) > 0
}

// SetBackground takes a frame of the empty channel as the background, replacing any
// views.
func (d *BackgroundDetector) SetBackground(frame image.Image) {
	d.views, d.lastView = nil, -1
	b := frame.Bounds()
	step := max(1, d.Step)
	d.bgW, d.bgH = (b.Dx()+step-1)/step, (b.Dy()+step-1)/step
	d.bg = make([]float64, d.bgW*d.bgH*3)
	for gy := 0; gy < d.bgH; gy++ {
		for gx := 0; gx < d.bgW; gx++ {
			r, g, bl := rgb(frame.At(b.Min.X+gx*step, b.Min.Y+gy*step))
			i := (gy*d.bgW + gx) * 3
			d.bg[i], d.bg[i+1], d.bg[i+2] = r, g, bl
		}
	}
	d.mask = nil
}

func (d *BackgroundDetector) channelMask(z ChannelZones) []bool {
	if d.mask != nil && d.maskOf == z {
		return d.mask
	}
	step := max(1, d.Step)
	mask := make([]bool, d.bgW*d.bgH)
	for gy := 0; gy < d.bgH; gy++ {
		for gx := 0; gx < d.bgW; gx++ {
			mask[gy*d.bgW+gx] = z.OnChannel(Point{X: float64(gx * step), Y: float64(gy * step)})
		}
	}
	d.mask, d.maskOf = mask, z
	return mask
}

// Detect returns a box per blob of changed pixels on the channel.
func (d *BackgroundDetector) Detect(frame image.Image, z ChannelZones) ([]Box, error) {
	if len(d.views) > 0 {
		return d.detectViews(frame, z)
	}
	if d.bg == nil {
		return nil, ErrNoBackground
	}
	b := frame.Bounds()
	step := max(1, d.Step)
	if (b.Dx()+step-1)/step != d.bgW || (b.Dy()+step-1)/step != d.bgH {
		return nil, errors.New("the frame size changed; capture the empty channel again")
	}
	mask := d.channelMask(z)
	changed := make([]bool, len(mask))
	for gy := 0; gy < d.bgH; gy++ {
		for gx := 0; gx < d.bgW; gx++ {
			k := gy*d.bgW + gx
			if !mask[k] {
				continue
			}
			r, g, bl := rgb(frame.At(b.Min.X+gx*step, b.Min.Y+gy*step))
			i := k * 3
			diff := abs(r-d.bg[i]) + abs(g-d.bg[i+1]) + abs(bl-d.bg[i+2])
			if diff >= float64(d.Threshold) {
				changed[k] = true
			} else if d.Learn > 0 {
				d.bg[i] += d.Learn * (r - d.bg[i])
				d.bg[i+1] += d.Learn * (g - d.bg[i+1])
				d.bg[i+2] += d.Learn * (bl - d.bg[i+2])
			}
		}
	}
	return d.blobs(changed, step, b.Min), nil
}

// blobs labels 8-connected regions of changed cells and returns their boxes.
func (d *BackgroundDetector) blobs(changed []bool, step int, origin image.Point) []Box {
	minCells := max(1, d.MinArea/(step*step))
	seen := make([]bool, len(changed))
	var out []Box
	queue := make([]int, 0, 256)
	for start, on := range changed {
		if !on || seen[start] {
			continue
		}
		seen[start] = true
		queue = append(queue[:0], start)
		x1, y1, x2, y2, cells := d.bgW, d.bgH, -1, -1, 0
		for len(queue) > 0 {
			k := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			cells++
			x, y := k%d.bgW, k/d.bgW
			x1, y1, x2, y2 = min(x1, x), min(y1, y), max(x2, x), max(y2, y)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= d.bgW || ny >= d.bgH {
						continue
					}
					n := ny*d.bgW + nx
					if changed[n] && !seen[n] {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
		}
		if cells < minCells {
			continue
		}
		out = append(out, Box{
			X1: origin.X + x1*step, Y1: origin.Y + y1*step,
			X2: origin.X + (x2+1)*step, Y2: origin.Y + (y2+1)*step,
		})
	}
	return out
}

// MeanColor is the average color inside a box, as 0..1 RGB, sampling every step-th
// pixel.
func MeanColor(frame image.Image, box Box, step int) [3]float64 {
	step = max(1, step)
	r := image.Rect(box.X1, box.Y1, box.X2, box.Y2).Intersect(frame.Bounds())
	var sum [3]float64
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y += step {
		for x := r.Min.X; x < r.Max.X; x += step {
			cr, cg, cb := rgb(frame.At(x, y))
			sum[0], sum[1], sum[2] = sum[0]+cr, sum[1]+cg, sum[2]+cb
			n++
		}
	}
	if n == 0 {
		return sum
	}
	return [3]float64{sum[0] / float64(n) / 255, sum[1] / float64(n) / 255, sum[2] / float64(n) / 255}
}

func rgb(c color.Color) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return float64(r >> 8), float64(g >> 8), float64(b >> 8)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
