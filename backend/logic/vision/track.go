package vision

import (
	"math"
	"sort"
	"time"
)

// Tracker gives pieces on a channel ids that stay the same from frame to frame.
// Pieces ride an arc, so it matches in angle, not box overlap: each track predicts
// its angle from its angular speed, and a detection near that prediction (and of a
// similar radius and color) continues it. After Sorter V2's AngularColorTracker.
type Tracker struct {
	AngularGateDeg float64       // how far from its predicted angle a track may jump
	RadiusGateFrac float64       // how much its radius may change, as a share
	ColorGate      float64       // largest mean-color distance (0..√3) to match
	MaxCoast       time.Duration // a track unseen this long is dropped
	Smoothing      float64       // weight of the newest speed and color, 0..1

	tracks map[int]*track
	nextID int
}

type track struct {
	angle, radius, omega float64 // omega: degrees per second
	color                [3]float64
	hasColor             bool
	last                 time.Time
}

// NewTracker uses Sorter V2's defaults.
func NewTracker() *Tracker {
	return &Tracker{AngularGateDeg: 14, RadiusGateFrac: 0.3, ColorGate: 0.25, MaxCoast: 2500 * time.Millisecond, Smoothing: 0.5}
}

// Reset forgets every track.
func (t *Tracker) Reset() {
	t.tracks = nil
}

// Update tracks one observation's pieces in place, setting each one's TrackID.
// colors holds each piece's mean color (0..1 RGB) in the same order, or is nil.
func (t *Tracker) Update(z ChannelZones, obs *Observation, colors [][3]float64) {
	if t.tracks == nil {
		t.tracks = make(map[int]*track)
		if t.nextID == 0 {
			t.nextID = 1
		}
	}
	now := obs.At
	type det struct {
		angle, radius float64
		color         [3]float64
		hasColor      bool
	}
	dets := make([]det, len(obs.Pieces))
	for i, p := range obs.Pieces {
		c := p.Box.Center()
		dets[i] = det{angle: p.Angle, radius: math.Hypot(c.X-z.Ring.Center.X, c.Y-z.Ring.Center.Y)}
		if i < len(colors) {
			dets[i].color, dets[i].hasColor = colors[i], true
		}
	}

	type cand struct {
		dist   float64
		id, di int
	}
	var cands []cand
	for id, tr := range t.tracks {
		dt := math.Max(1e-3, now.Sub(tr.last).Seconds())
		predicted := NormalizeAngle(tr.angle + tr.omega*dt)
		rTol := math.Max(1, t.RadiusGateFrac*math.Max(tr.radius, 1))
		for di, d := range dets {
			ad := angleDist(predicted, d.angle)
			if ad > t.AngularGateDeg || math.Abs(d.radius-tr.radius) > rTol {
				continue
			}
			if tr.hasColor && d.hasColor && colorDist(tr.color, d.color) > t.ColorGate {
				continue
			}
			cands = append(cands, cand{ad, id, di})
		}
	}
	// Greedy, nearest first: a channel holds only a few pieces.
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].id < cands[j].id
	})
	usedTrack := map[int]bool{}
	assigned := make([]int, len(dets))
	for _, c := range cands {
		if usedTrack[c.id] || assigned[c.di] != 0 {
			continue
		}
		usedTrack[c.id] = true
		assigned[c.di] = c.id
	}

	a := t.Smoothing
	for di, id := range assigned {
		d := dets[di]
		if id == 0 {
			id = t.nextID
			t.nextID++
			t.tracks[id] = &track{angle: d.angle, radius: d.radius, color: d.color, hasColor: d.hasColor, last: now}
			assigned[di] = id
			continue
		}
		tr := t.tracks[id]
		dt := math.Max(1e-3, now.Sub(tr.last).Seconds())
		tr.omega = a*(signedAngleDelta(tr.angle, d.angle)/dt) + (1-a)*tr.omega
		tr.angle, tr.radius, tr.last = d.angle, d.radius, now
		if d.hasColor {
			if tr.hasColor {
				for i := range tr.color {
					tr.color[i] = a*d.color[i] + (1-a)*tr.color[i]
				}
			} else {
				tr.color, tr.hasColor = d.color, true
			}
		}
	}
	for id, tr := range t.tracks {
		if now.Sub(tr.last) > t.MaxCoast {
			delete(t.tracks, id)
		}
	}
	for i := range obs.Pieces {
		obs.Pieces[i].TrackID = assigned[i]
	}
}

func angleDist(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	return math.Min(d, 360-d)
}

// signedAngleDelta is the shortest turn from one angle to another, in (-180, 180].
func signedAngleDelta(from, to float64) float64 {
	d := NormalizeAngle(to - from)
	if d > 180 {
		d -= 360
	}
	return d
}

func colorDist(a, b [3]float64) float64 {
	return math.Sqrt((a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]) + (a[2]-b[2])*(a[2]-b[2]))
}
