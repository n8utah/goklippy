package sorter

import (
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
// one detection is ignored each frame, and only one that matches it, lying where it
// lies (its box overlapping the spot's by ignoreMinOverlap) and of its size (its blobs'
// area within ignoreAreaRatio). A piece landing next to it is a second detection and
// is seen; one landing on it merges into a bigger blob and is seen.

const (
	ignoreMinOverlap = 0.5 // intersection over union of the boxes
	ignoreAreaRatio  = 1.6 // the blobs' area may be this much larger or smaller
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
	box      vision.Box
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
	s[ch] = append(s[ch], &ignoredSpot{box: box, area: area, lastSeen: now})
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
		best, bestOverlap := -1, ignoreMinOverlap
		for i, g := range groups {
			if used[i] || !similarArea(partsArea(g), sp.area) {
				continue
			}
			if v := overlap(g.Box, sp.box); v >= bestOverlap {
				best, bestOverlap = i, v
			}
		}
		if best >= 0 {
			used[best] = true
			sp.lastSeen = at
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

// overlap is the boxes' intersection over their union.
func overlap(a, b vision.Box) float64 {
	in := vision.Box{X1: max(a.X1, b.X1), Y1: max(a.Y1, b.Y1), X2: min(a.X2, b.X2), Y2: min(a.Y2, b.Y2)}.Area()
	union := a.Area() + b.Area() - in
	if union <= 0 {
		return 0
	}
	return float64(in) / float64(union)
}
