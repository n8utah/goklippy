package vision

import (
	"errors"
	"image"
	"math"
	"math/rand"
)

// MeasureRing finds a channel's ring in a picture of it: the bowl's rim (bright bowl
// against dark surroundings) and the hole in its middle. It's how the Cameras page
// draws a channel's ring in one click; the drop and exit zones are left to the person.

// RingMeasure is what MeasureRing found.
type RingMeasure struct {
	Ring Ring `json:"ring"`
	// Rim and Hole are the fitted circles, before margins.
	RimCenter  Point   `json:"rim_center"`
	RimRadius  float64 `json:"rim_radius"`
	HoleCenter Point   `json:"hole_center"`
	HoleRadius float64 `json:"hole_radius"`
	// RimFit and HoleFit are the share of edge points on each circle (0–1); low
	// means the circle is a guess.
	RimFit  float64 `json:"rim_fit"`
	HoleFit float64 `json:"hole_fit"`
	// OutOfFrame is the share of the rim's circumference outside the picture. Pieces
	// riding there can't be seen.
	OutOfFrame float64 `json:"out_of_frame"`
	Threshold  int     `json:"threshold"` // the brightness splitting bowl from surroundings
}

var ErrNoRing = errors.New("couldn't find the channel's rim in the picture: is the bowl lit and in view?")

const (
	measureRun        = 4    // bright pixels in a row to count as the bowl's edge
	measureInlier     = 4.0  // pixels from the circle to count as on it
	measureIterations = 400  // RANSAC tries
	measureMinFit     = 0.35 // below this share of points on the circle, give up
	// The ring's edges from the circles: just outside the hole's wall, and a little
	// past the rim, where pieces ride against the wall.
	innerMargin = 6.0
	outerGrow   = 1.04
)

// MeasureRing fits the rim and the hole. It needs the bowl brighter than what's
// around it, the usual case for a white bowl.
func MeasureRing(img image.Image) (RingMeasure, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	lum := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum[y*w+x] = uint8((299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000)
		}
	}
	at := func(x, y int) uint8 { return lum[y*w+x] }
	// The rim is where the bowl meets the surroundings, the darkest part of the
	// picture. The bowl itself can be half in shadow (no light yet), so the split is a
	// sixth of the way from the surroundings' brightness to the bowl's bright side,
	// not the picture's best two-way split (that splits the bowl's light and shadow).
	dark, light := percentile(lum, 0.05), percentile(lum, 0.9)
	thr := uint8(max(int(dark)+8, int(dark)+(int(light)-int(dark))/6))
	bright := func(x, y int) bool { return at(x, y) >= thr }

	// Rim edge points: the first run of bright pixels coming in from each side, along
	// every few rows and columns. A run that starts at the frame's edge isn't an edge.
	var rim [][2]float64
	scan := func(n int, pos func(i int) (int, int), length int) {
		for i := 0; i <= length-measureRun; i++ {
			ok := true
			for k := 0; k < measureRun; k++ {
				if x, y := pos(i + k); !bright(x, y) {
					ok = false
					break
				}
			}
			if ok {
				if i > 1 {
					x, y := pos(i)
					rim = append(rim, [2]float64{float64(x), float64(y)})
				}
				return
			}
		}
	}
	for y := 0; y < h; y += 3 {
		scan(w, func(i int) (int, int) { return i, y }, w)
		scan(w, func(i int) (int, int) { return w - 1 - i, y }, w)
	}
	for x := 0; x < w; x += 3 {
		scan(h, func(i int) (int, int) { return x, i }, h)
		scan(h, func(i int) (int, int) { return x, h - 1 - i }, h)
	}
	rc, rr, rfit := fitCircleRobust(rim, float64(max(w, h)))
	if rfit < measureMinFit || rr < float64(min(w, h))/8 {
		return RingMeasure{}, ErrNoRing
	}

	// Hole: from the rim's center outward along rays, the first bright pixel after the
	// dark middle, with its own split worked out from the middle of the bowl. Rays
	// through something dark (a bracket) end farther out and are left out by the fit.
	var middle []uint8
	for y := max(0, int(rc.Y-rr*0.4)); y < min(h, int(rc.Y+rr*0.4)); y++ {
		for x := max(0, int(rc.X-rr*0.4)); x < min(w, int(rc.X+rr*0.4)); x++ {
			middle = append(middle, at(x, y))
		}
	}
	holeThr := otsu(middle)
	bright = func(x, y int) bool { return at(x, y) >= holeThr }
	var hole [][2]float64
	for a := 0.0; a < 360; a += 3 {
		ra := a * math.Pi / 180
		dark := false
		for d := 0.0; d < rr*0.6; d++ {
			x, y := int(rc.X+d*math.Cos(ra)), int(rc.Y+d*math.Sin(ra))
			if x < 0 || y < 0 || x >= w || y >= h {
				break
			}
			if !bright(x, y) {
				dark = true
			} else if dark {
				hole = append(hole, [2]float64{float64(x), float64(y)})
				break
			}
		}
	}
	m := RingMeasure{RimCenter: rc, RimRadius: rr, RimFit: rfit, Threshold: int(thr)}
	m.OutOfFrame = outOfFrame(rc, rr, w, h)
	inner := rr * 0.2 // no hole found: a fifth of the rim
	if hc, hr, hfit := fitCircleRobust(hole, rr); hfit >= measureMinFit && hr < rr/2 {
		m.HoleCenter, m.HoleRadius, m.HoleFit = hc, hr, hfit
		// The hole may sit off the rim's center (the camera's tilt): reach its far side.
		inner = math.Hypot(hc.X-rc.X, hc.Y-rc.Y) + hr + innerMargin
	}
	outer := rr * outerGrow
	m.Ring = Ring{
		Center:      Point{math.Round(rc.X), math.Round(rc.Y)},
		InnerRadius: math.Round(inner),
		OuterRadius: math.Round(outer),
		ExitRadius:  math.Round(rr),
	}
	return m, nil
}

// fitCircleRobust fits a circle to points that include strays: random triples propose
// circles, the one most points sit on wins, and it's refined on those points. It
// returns the share of points on it.
func fitCircleRobust(pts [][2]float64, maxR float64) (Point, float64, float64) {
	if len(pts) < 8 {
		return Point{}, 0, 0
	}
	rng := rand.New(rand.NewSource(1)) // the same picture always gives the same ring
	bestN, bestC, bestR := 0, Point{}, 0.0
	for it := 0; it < measureIterations; it++ {
		a, b, c := pts[rng.Intn(len(pts))], pts[rng.Intn(len(pts))], pts[rng.Intn(len(pts))]
		cx, cy, r, ok := circleThrough(a, b, c)
		if !ok || r < 5 || r > maxR*1.5 {
			continue
		}
		n := 0
		for _, p := range pts {
			if math.Abs(math.Hypot(p[0]-cx, p[1]-cy)-r) < measureInlier {
				n++
			}
		}
		if n > bestN {
			bestN, bestC, bestR = n, Point{cx, cy}, r
		}
	}
	if bestN == 0 {
		return Point{}, 0, 0
	}
	var in [][2]float64
	for _, p := range pts {
		if math.Abs(math.Hypot(p[0]-bestC.X, p[1]-bestC.Y)-bestR) < measureInlier {
			in = append(in, p)
		}
	}
	cx, cy, r := fitCircleLS(in)
	return Point{cx, cy}, r, float64(len(in)) / float64(len(pts))
}

func circleThrough(a, b, c [2]float64) (float64, float64, float64, bool) {
	d := 2 * (a[0]*(b[1]-c[1]) + b[0]*(c[1]-a[1]) + c[0]*(a[1]-b[1]))
	if math.Abs(d) < 1e-9 {
		return 0, 0, 0, false
	}
	a2, b2, c2 := a[0]*a[0]+a[1]*a[1], b[0]*b[0]+b[1]*b[1], c[0]*c[0]+c[1]*c[1]
	cx := (a2*(b[1]-c[1]) + b2*(c[1]-a[1]) + c2*(a[1]-b[1])) / d
	cy := (a2*(c[0]-b[0]) + b2*(a[0]-c[0]) + c2*(b[0]-a[0])) / d
	return cx, cy, math.Hypot(a[0]-cx, a[1]-cy), true
}

// fitCircleLS is a least-squares (Kåsa) circle fit.
func fitCircleLS(pts [][2]float64) (float64, float64, float64) {
	var m [3][4]float64
	for _, p := range pts {
		x, y := p[0], p[1]
		z := x*x + y*y
		row := [3]float64{x, y, 1}
		rhs := [3]float64{x * z, y * z, z}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				m[i][j] += row[i] * row[j]
			}
			m[i][3] += rhs[i]
		}
	}
	for i := 0; i < 3; i++ { // Gauss-Jordan with partial pivoting
		p := i
		for k := i + 1; k < 3; k++ {
			if math.Abs(m[k][i]) > math.Abs(m[p][i]) {
				p = k
			}
		}
		m[i], m[p] = m[p], m[i]
		if m[i][i] == 0 {
			return 0, 0, 0
		}
		for k := 0; k < 3; k++ {
			if k != i {
				f := m[k][i] / m[i][i]
				for j := i; j < 4; j++ {
					m[k][j] -= f * m[i][j]
				}
			}
		}
	}
	a, b, c := m[0][3]/m[0][0], m[1][3]/m[1][1], m[2][3]/m[2][2]
	cx, cy := a/2, b/2
	return cx, cy, math.Sqrt(c + cx*cx + cy*cy)
}

// percentile is the brightness below which share p of the pixels fall.
func percentile(lum []uint8, p float64) uint8 {
	var hist [256]int
	for _, v := range lum {
		hist[v]++
	}
	want := int(p * float64(len(lum)))
	seen := 0
	for i, n := range hist {
		seen += n
		if seen > want {
			return uint8(i)
		}
	}
	return 255
}

// otsu is the brightness that best splits the picture into dark and bright.
func otsu(lum []uint8) uint8 {
	var hist [256]float64
	for _, v := range lum {
		hist[v]++
	}
	total := float64(len(lum))
	var sum float64
	for i, n := range hist {
		sum += float64(i) * n
	}
	var sumB, wB, best float64
	thr := uint8(128)
	for i := 0; i < 256; i++ {
		wB += hist[i]
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += float64(i) * hist[i]
		mB, mF := sumB/wB, (sum-sumB)/wF
		if between := wB * wF * (mB - mF) * (mB - mF); between > best {
			best, thr = between, uint8(i+1)
		}
	}
	return thr
}

// outOfFrame is the share of a circle's circumference outside a w×h picture.
func outOfFrame(c Point, r float64, w, h int) float64 {
	out := 0
	const n = 360
	for i := 0; i < n; i++ {
		a := float64(i) * math.Pi / 180
		x, y := c.X+r*math.Cos(a), c.Y+r*math.Sin(a)
		if x < 0 || y < 0 || x >= float64(w) || y >= float64(h) {
			out++
		}
	}
	return float64(out) / n
}
