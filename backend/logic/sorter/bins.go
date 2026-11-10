package sorter

import (
	"slices"

	"goklippy/backend/logic/sorter/profile"
)

// Which bin a piece goes to, after Sorter V2 (subsystems/distribution/positioning.py,
// _findOrAssignBinForCategory). A profile never names bins: the machine gives each
// category a bin the first time it needs one.

const (
	// MaxPieceMm is the longest piece any bin takes (10 studs); longer pieces drop
	// into the bucket.
	MaxPieceMm = 80.0
)

// layerMaxPieceDefaults is the longest piece a layer's funnels take, by bin count:
// denser layers have narrower funnels.
var layerMaxPieceDefaults = map[int]float64{6: 80, 12: 80, 18: 50, 30: 40}

// BinKey names a bin by layer and bin number, both from 1.
type BinKey struct {
	Layer int `json:"layer"`
	Bin   int `json:"bin"`
}

// Assignments is which categories each bin takes, in the order they were given.
type Assignments map[BinKey][]string

// Clone copies the assignments so a caller can change them.
func (a Assignments) Clone() Assignments {
	out := make(Assignments, len(a))
	for k, v := range a {
		out[k] = slices.Clone(v)
	}
	return out
}

// Where a piece went and why.
const (
	RouteAssignedBin = "assigned_bin" // the category already had this bin
	RouteNewBin      = "new_bin"      // the category was given an empty bin
	RouteSharedBin   = "shared_bin"   // the category joined a bin with others
	RouteMiscBin     = "misc_bin"     // no bin for the category; a bin taking misc
	RouteMisc        = "misc"         // misc, with no misc bin: the bucket
	RouteTooBig      = "too_big"      // longer than any bin takes: the bucket
	RouteNoBin       = "no_bin"       // every bin is taken: the bucket
)

// BinChoice is where one piece goes. Bin is nil for the bucket under the tower.
type BinChoice struct {
	Bin    *BinPosition `json:"bin,omitempty"`
	Reason string       `json:"reason"`
	// Assigned is true when Bin was newly given Category; the caller saves it.
	Assigned bool `json:"assigned"`
}

// LayerMaxPieceMm is the longest piece a layer takes; 0 means no limit.
func (l LayerSettings) LayerMaxPieceMm() float64 {
	if l.MaxPieceMm > 0 {
		return l.MaxPieceMm
	}
	return layerMaxPieceDefaults[l.Bins]
}

// usableBins lists the bins a piece of pieceMm (0 = unknown) can go to: enabled
// layers and sections, reachable by the chute, with funnels wide enough.
func (d DistributorSettings) usableBins(pieceMm float64) []BinPosition {
	var out []BinPosition
	for l, layerBins := range d.LayerBins() {
		layer := d.Layers[l]
		if layer.Disabled {
			continue
		}
		if limit := layer.LayerMaxPieceMm(); pieceMm > 0 && limit > 0 && pieceMm > limit {
			continue
		}
		for _, pos := range layerBins {
			if pos.Reachable && !slices.Contains(layer.DisabledSections, pos.Section) {
				out = append(out, pos)
			}
		}
	}
	return out
}

// ChooseBin picks the bin for a piece of category, given the bins' current
// categories. pick(n) returns a number in [0, n); it chooses among several bins a
// category has, so they fill evenly. When the choice gives a bin a new category,
// newCategories is that bin's full category list for the caller to save.
func (d DistributorSettings) ChooseBin(assigned Assignments, category string, pieceMm float64, pick func(n int) int) (choice BinChoice, newCategories []string) {
	if pieceMm > MaxPieceMm {
		return BinChoice{Reason: RouteTooBig}, nil
	}
	usable := d.usableBins(pieceMm)

	var matching []BinPosition
	var firstEmpty *BinPosition
	var leastShared *BinPosition
	for i := range usable {
		pos := usable[i]
		cats := assigned[BinKey{pos.Layer, pos.Bin}]
		switch {
		case slices.Contains(cats, category):
			matching = append(matching, pos)
		case len(cats) == 0:
			if firstEmpty == nil {
				firstEmpty = &usable[i]
			}
		case !slices.Contains(cats, profile.Misc):
			// Fewest categories first, then the earliest bin.
			if leastShared == nil || len(cats) < len(assigned[BinKey{leastShared.Layer, leastShared.Bin}]) {
				leastShared = &usable[i]
			}
		}
	}

	if len(matching) > 0 {
		pos := matching[pick(len(matching))]
		reason := RouteAssignedBin
		if category == profile.Misc {
			reason = RouteMiscBin
		}
		return BinChoice{Bin: &pos, Reason: reason}, nil
	}
	if category == profile.Misc {
		// Misc never claims a bin: it falls through to the bucket unless a bin was
		// given misc by hand.
		return BinChoice{Reason: RouteMisc}, nil
	}
	if firstEmpty != nil {
		pos := *firstEmpty
		return BinChoice{Bin: &pos, Reason: RouteNewBin, Assigned: true}, []string{category}
	}
	if d.SharedBins && leastShared != nil {
		pos := *leastShared
		cats := append(slices.Clone(assigned[BinKey{pos.Layer, pos.Bin}]), category)
		return BinChoice{Bin: &pos, Reason: RouteSharedBin, Assigned: true}, cats
	}
	// No bin for the category: a misc bin if there is one, else the bucket.
	if choice, _ := d.ChooseBin(assigned, profile.Misc, pieceMm, pick); choice.Bin != nil {
		return choice, nil
	}
	return BinChoice{Reason: RouteNoBin}, nil
}
