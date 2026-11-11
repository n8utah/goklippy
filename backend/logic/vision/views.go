package vision

import (
	"errors"
	"image"
	"math"
	"sort"
)

// Views: empty-channel pictures taken all the way round a channel whose rotor has a
// pattern on it (the cone's facets). A single picture only matches the rotor where it
// was taken; as it turns, the pattern's edges show up as pieces. With a view every
// couple of degrees, each frame is compared with the view that matches it best, so
// the pattern cancels out wherever the rotor stands. The view is picked by comparing
// pictures, not from the motor's position, so it survives a board reset.

const (
	// viewDiffCap caps a pixel's difference when matching views, so pieces (big
	// differences in a few places) don't sway which view matches.
	viewDiffCap = 60
	// viewCoarseStride samples every Nth channel pixel for the first, cheap pass over
	// all views; the best viewShortlist are then compared on every channel pixel.
	viewCoarseStride = 7
	viewShortlist    = 3
)

var errViewSize = errors.New("the empty-channel pictures are a different size from the camera's; capture them again")

// SetViews takes frames of the empty channel all the way round as the backgrounds,
// replacing any single background picture.
func (d *BackgroundDetector) SetViews(frames []image.Image) error {
	if len(frames) == 0 {
		d.views = nil
		return nil
	}
	views := make([][]uint8, len(frames))
	var w, h int
	for i, f := range frames {
		v, vw, vh := d.sample(f)
		if i > 0 && (vw != w || vh != h) {
			return errViewSize
		}
		views[i], w, h = v, vw, vh
	}
	d.views, d.bgW, d.bgH = views, w, h
	d.bg, d.mask, d.lastView = nil, nil, -1
	return nil
}

// Views is how many all-the-way-round pictures the detector has (0 = none).
func (d *BackgroundDetector) Views() int { return len(d.views) }

// LastView is the view the last frame matched best, -1 if none.
func (d *BackgroundDetector) LastView() int { return d.lastView }

// sample reads every Step-th pixel's RGB.
func (d *BackgroundDetector) sample(frame image.Image) ([]uint8, int, int) {
	b := frame.Bounds()
	step := max(1, d.Step)
	w, h := (b.Dx()+step-1)/step, (b.Dy()+step-1)/step
	out := make([]uint8, w*h*3)
	for gy := 0; gy < h; gy++ {
		for gx := 0; gx < w; gx++ {
			r, g, bl, _ := frame.At(b.Min.X+gx*step, b.Min.Y+gy*step).RGBA()
			i := (gy*w + gx) * 3
			out[i], out[i+1], out[i+2] = uint8(r>>8), uint8(g>>8), uint8(bl>>8)
		}
	}
	return out, w, h
}

// matchView picks the view most like this frame on the channel's pixels.
func (d *BackgroundDetector) matchView(px []uint8, mask []bool) int {
	var cells []int
	for k, on := range mask {
		if on {
			cells = append(cells, k)
		}
	}
	score := func(v []uint8, stride int) int {
		total := 0
		for j := 0; j < len(cells); j += stride {
			i := cells[j] * 3
			diff := absInt(int(px[i])-int(v[i])) + absInt(int(px[i+1])-int(v[i+1])) + absInt(int(px[i+2])-int(v[i+2]))
			total += min(diff, viewDiffCap)
		}
		return total
	}
	type cand struct{ view, score int }
	coarse := make([]cand, len(d.views))
	for i, v := range d.views {
		coarse[i] = cand{i, score(v, viewCoarseStride)}
	}
	sort.Slice(coarse, func(a, b int) bool { return coarse[a].score < coarse[b].score })
	best, bestScore := coarse[0].view, -1
	for _, c := range coarse[:min(viewShortlist, len(coarse))] {
		if s := score(d.views[c.view], 1); bestScore < 0 || s < bestScore {
			best, bestScore = c.view, s
		}
	}
	return best
}

// detectViews is Detect against the best-matching view.
func (d *BackgroundDetector) detectViews(frame image.Image, z ChannelZones) ([]Box, error) {
	px, w, h := d.sample(frame)
	if w != d.bgW || h != d.bgH {
		return nil, errViewSize
	}
	mask := d.channelMask(z)
	d.lastView = d.matchView(px, mask)
	view := d.views[d.lastView]
	slack := d.viewSlack(z)
	changed := make([]bool, len(mask))
	for k, on := range mask {
		if !on {
			continue
		}
		i := k * 3
		gx, gy := k%d.bgW, k/d.bgW
		// Changed only if it differs from every view pixel within the slack: the rotor
		// can sit up to half a view's spacing from the nearest view, which moves the
		// pattern's edges a few pixels. A piece differs from all of them.
		least := -1
		for dy := -slack; dy <= slack && least != 0; dy++ {
			for dx := -slack; dx <= slack; dx++ {
				nx, ny := gx+dx, gy+dy
				if nx < 0 || ny < 0 || nx >= d.bgW || ny >= d.bgH {
					continue
				}
				j := (ny*d.bgW + nx) * 3
				diff := absInt(int(px[i])-int(view[j])) + absInt(int(px[i+1])-int(view[j+1])) + absInt(int(px[i+2])-int(view[j+2]))
				if least < 0 || diff < least {
					least = diff
				}
			}
		}
		changed[k] = least >= d.Threshold
	}
	return d.blobs(changed, max(1, d.Step), frame.Bounds().Min), nil
}

// viewSlack is how many sample cells the pattern can be off from the nearest view:
// half the angle between views, at the ring's outer edge, plus a pixel.
func (d *BackgroundDetector) viewSlack(z ChannelZones) int {
	spacing := 360 / float64(len(d.views)) * math.Pi / 180
	px := z.Ring.OuterRadius*spacing/2 + 1
	return max(1, int(math.Ceil(px/float64(max(1, d.Step)))))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
