package vision

import (
	"sort"
	"time"
)

// PieceObs is one piece on a channel in one frame.
type PieceObs struct {
	Box   Box     `json:"box"`
	Angle float64 `json:"angle"` // of the box's center
	Zone  Zone    `json:"zone"`  // the zone its center is in
	// ForwardToExit is how far its center is short of the exit (≤ 0: in it).
	ForwardToExit float64 `json:"forward_to_exit"`
	// TrackID is the tracker's id for it, steady across frames; 0 until tracked.
	TrackID int `json:"track_id,omitempty"`
}

// Observation is everything the sorter reads about one channel from one frame.
type Observation struct {
	At time.Time `json:"at"`
	// Valid is false until the channel's camera has produced a frame.
	Valid bool `json:"valid"`
	// A zone holds a piece if any part of a box (any of 9 sample points) is in it.
	InDrop    bool `json:"in_drop"`
	InPrecise bool `json:"in_precise"`
	InExit    bool `json:"in_exit"`
	// Pieces on the channel, leading (nearest the exit) first.
	Pieces []PieceObs `json:"pieces"`
	// Merged lists boxes that were joined from several of the detector's boxes:
	// one piece in parts, or two pieces touching.
	Merged []MergedBox `json:"merged,omitempty"`
}

// Leading is the piece nearest the exit, if any.
func (o Observation) Leading() (PieceObs, bool) {
	if len(o.Pieces) == 0 {
		return PieceObs{}, false
	}
	return o.Pieces[0], true
}

// Empty reports a valid frame with nothing on the channel.
func (o Observation) Empty() bool {
	return o.Valid && len(o.Pieces) == 0
}

// MergedBox is a box made from several detections.
type MergedBox struct {
	Box   Box   `json:"box"`
	Parts []Box `json:"parts"`
}

// Observe turns a frame's detections into an observation of the channel. Boxes off
// the channel are dropped; boxes within mergeGap pixels of each other become one.
func Observe(z ChannelZones, boxes []Box, mergeGap int, at time.Time) Observation {
	obs := Observation{At: at, Valid: true, Pieces: []PieceObs{}}
	for _, m := range MergeBoxes(boxes, mergeGap) {
		center := m.Box.Center()
		if !z.OnChannel(center) {
			continue
		}
		if len(m.Parts) > 1 {
			obs.Merged = append(obs.Merged, m)
		}
		for _, p := range m.Box.samples() {
			if !z.OnChannel(p) {
				continue
			}
			switch z.ZoneAt(z.AngleOf(p)) {
			case ZoneDrop:
				obs.InDrop = true
			case ZonePrecise:
				obs.InPrecise = true
			case ZoneExit:
				obs.InExit = true
			}
		}
		angle := z.AngleOf(center)
		obs.Pieces = append(obs.Pieces, PieceObs{
			Box: m.Box, Angle: angle, Zone: z.ZoneAt(angle), ForwardToExit: z.ForwardToExit(angle),
		})
	}
	sort.SliceStable(obs.Pieces, func(i, j int) bool { return obs.Pieces[i].ForwardToExit < obs.Pieces[j].ForwardToExit })
	return obs
}

// MergeBoxes joins boxes that overlap or sit within gap pixels of each other
// (transitively): a detector often draws one box per color region of a piece. Each
// result has its union box and the boxes it came from.
func MergeBoxes(boxes []Box, gap int) []MergedBox {
	n := len(boxes)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	near := func(a, b Box) bool {
		return a.X1-gap <= b.X2 && b.X1-gap <= a.X2 && a.Y1-gap <= b.Y2 && b.Y1-gap <= a.Y2
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if near(boxes[i], boxes[j]) {
				parent[find(i)] = find(j)
			}
		}
	}
	groups := map[int]*MergedBox{}
	var order []int
	for i, b := range boxes {
		root := find(i)
		g, ok := groups[root]
		if !ok {
			g = &MergedBox{Box: b}
			groups[root] = g
			order = append(order, root)
		} else {
			g.Box = Box{X1: min(g.Box.X1, b.X1), Y1: min(g.Box.Y1, b.Y1), X2: max(g.Box.X2, b.X2), Y2: max(g.Box.Y2, b.Y2)}
		}
		g.Parts = append(g.Parts, b)
	}
	out := make([]MergedBox, 0, len(order))
	for _, root := range order {
		out = append(out, *groups[root])
	}
	return out
}
