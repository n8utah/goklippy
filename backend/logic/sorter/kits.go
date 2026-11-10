package sorter

import (
	"sort"

	"goklippy/backend/logic/sorter/profile"
)

// kitTracker counts the pieces each kit (a set profile's rule) has taken, so a kit
// passes a part on to the next rule once it has enough. After Sorter V2's
// set_progress.py.
type kitTracker struct {
	lines map[string][]profile.KitLine
	found map[kitKey]int
}

type kitKey struct {
	category, part, color string // color "" is a line for any color
}

func newKitTracker(lines map[string][]profile.KitLine) *kitTracker {
	return &kitTracker{lines: lines, found: map[kitKey]int{}}
}

// matching lines for a part in a color: the color's own line first, then any color's.
func (k *kitTracker) matching(category, part, color string) []profile.KitLine {
	var exact, anyColor []profile.KitLine
	for _, l := range k.lines[category] {
		switch {
		case l.Part != part:
		case l.Color == "":
			anyColor = append(anyColor, l)
		case l.Color == color:
			exact = append(exact, l)
		}
	}
	return append(exact, anyColor...)
}

// record counts a piece the kit took against its first line still short.
func (k *kitTracker) record(category, part, color string) {
	for _, l := range k.matching(category, part, color) {
		key := kitKey{category, l.Part, l.Color}
		if k.found[key] < l.Need {
			k.found[key]++
			return
		}
	}
}

// full reports a kit that has every piece of this part and color it needs. A part
// the kit doesn't list never blocks it.
func (k *kitTracker) full(category, part, color string) bool {
	lines := k.matching(category, part, color)
	if len(lines) == 0 {
		return false
	}
	for _, l := range lines {
		if k.found[kitKey{category, l.Part, l.Color}] < l.Need {
			return false
		}
	}
	return true
}

// KitProgress is how far one kit has got.
type KitProgress struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Need     int    `json:"need"`
	Found    int    `json:"found"`
}

func (k *kitTracker) progress(name func(string) string) []KitProgress {
	out := []KitProgress{}
	for category, lines := range k.lines {
		p := KitProgress{Category: category, Name: name(category)}
		for _, l := range lines {
			p.Need += l.Need
			p.Found += min(l.Need, k.found[kitKey{category, l.Part, l.Color}])
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
