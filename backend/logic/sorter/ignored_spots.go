package sorter

import (
	"math"
	"time"

	"goklippy/backend/logic/vision"
)

// Working around a stuck piece. A piece caught on a fixed part (a rim, a guide) that
// shaking can't free would otherwise stop the sort until someone takes it off. With
// Feeder.WorkAroundStuck the machine instead leaves that one piece out of what the
// camera reports, notes it, and carries on: every other piece is still seen and
// sorted. Once the piece hasn't been seen for ignoreForgetAfter (it came free or was
// taken off) the spot is watched again.
//
// Only the stuck piece is left out, never "anything in that area": per spot, at most
// one detection is ignored each frame, the one nearest where the piece was last seen,
// and only if it's the piece's size (its blobs' area within ignoreAreaRatio) and close
// (its center within ignoreFollow of the piece's size from where it was last seen).
// A piece landing next to it is a second detection and is seen; one landing on it
// merges into a bigger blob and is seen. Shaking and pushing nudge a stuck piece, so
// the spot follows it, but never further than ignoreDrift from where it first stuck:
// a piece riding the channel soon leaves that and is seen.

const (
	// ignoreAreaRatio: the blobs' area may be this much larger or smaller. Under 2, so
	// two pieces lying together never pass for one.
	ignoreAreaRatio = 1.6
	ignoreFollow    = 0.6 // how far it may move between frames, in piece sizes
	ignoreDrift     = 1.5 // how far it may wander from where it stuck, in piece sizes
	// ignoreForgetAfter: a spot whose piece hasn't been seen this long is watched again.
	ignoreForgetAfter = 10 * time.Second
)

// SpotIgnorer is vision that can leave a stuck piece out of what it reports.
type SpotIgnorer interface {
	// IgnoreSpot stops reporting the piece seen at box (frame pixels) on ch, whose
	// blobs cover area pixels.
	IgnoreSpot(ch Channel, box vision.Box, area int)
	// ForgetIgnored watches all of ch again.
	ForgetIgnored(ch Channel)
}

type ignoredSpot struct {
	anchor   vision.Box // where it first stuck
	box      vision.Box // where it was last seen
	area     int
	lastSeen time.Time
}

// spotIgnores is each channel's ignored spots. Not safe for concurrent use: the
// owner locks around it.
type spotIgnores map[Channel][]*ignoredSpot

func (s spotIgnores) add(ch Channel, box vision.Box, area int, now time.Time) {
	if area <= 0 {
		area = box.Area()
	}
	s[ch] = append(s[ch], &ignoredSpot{anchor: box, box: box, area: area, lastSeen: now})
}

// filter takes a frame's detections and leaves out each spot's stuck piece, grouping
// them as Observe will (gap pixels). It forgets spots not seen for ignoreForgetAfter,
// and returns the detections kept and the spots still ignored.
func (s spotIgnores) filter(ch Channel, boxes []vision.Box, gap int, at time.Time) (kept, ignored []vision.Box) {
	spots := s[ch]
	if len(spots) == 0 {
		return boxes, nil
	}
	groups := vision.MergeBoxes(boxes, gap)
	used := make([]bool, len(groups))
	for _, sp := range spots {
		size := boxSize(sp.box)
		best, bestDist := -1, ignoreFollow*size
		for i, g := range groups {
			if used[i] || !similarArea(partsArea(g), sp.area) {
				continue
			}
			if centerDist(g.Box, sp.anchor) > ignoreDrift*boxSize(sp.anchor) {
				continue
			}
			if d := centerDist(g.Box, sp.box); d <= bestDist {
				best, bestDist = i, d
			}
		}
		if best >= 0 {
			used[best] = true
			sp.box, sp.area, sp.lastSeen = groups[best].Box, partsArea(groups[best]), at
		}
	}
	for i, g := range groups {
		if !used[i] {
			kept = append(kept, g.Parts...)
		}
	}
	live := spots[:0]
	for _, sp := range spots {
		if at.Sub(sp.lastSeen) < ignoreForgetAfter {
			live = append(live, sp)
			ignored = append(ignored, sp.box)
		}
	}
	if len(live) == 0 {
		delete(s, ch)
	} else {
		s[ch] = live
	}
	return kept, ignored
}

// pieceArea is how many pixels the blobs of the piece seen at box cover.
func pieceArea(obs vision.Observation, box vision.Box) int {
	for _, m := range obs.Merged {
		if m.Box == box {
			return partsArea(m)
		}
	}
	return box.Area()
}

func partsArea(m vision.MergedBox) int {
	if len(m.Parts) == 0 {
		return m.Box.Area()
	}
	n := 0
	for _, p := range m.Parts {
		n += p.Area()
	}
	return n
}

func similarArea(a, b int) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	r := float64(a) / float64(b)
	return r <= ignoreAreaRatio && r >= 1/ignoreAreaRatio
}

// boxSize is a box's larger side, at least one pixel.
func boxSize(b vision.Box) float64 {
	return float64(max(1, b.X2-b.X1, b.Y2-b.Y1))
}

func centerDist(a, b vision.Box) float64 {
	ca, cb := a.Center(), b.Center()
	return math.Hypot(ca.X-cb.X, ca.Y-cb.Y)
}

// stillThere reports a piece in obs where the stuck one was (box), of its size (area):
// after trying to push it on, it hasn't gone anywhere.
func stillThere(obs vision.Observation, box vision.Box, area int) bool {
	for _, p := range obs.Pieces {
		if similarArea(pieceArea(obs, p.Box), area) && centerDist(p.Box, box) <= 0.5*boxSize(box) {
			return true
		}
	}
	return false
}
