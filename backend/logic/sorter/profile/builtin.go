package profile

import "strings"

// Built-in profiles need no file: they sort by what Brickognize says about a piece.
// They're stored as a tiny JSON document, {"builtin": "<kind>"}.
const (
	// BuiltinPartType sorts by part type: Brickognize's category up to the first comma,
	// so "Plate, Round" and "Plate, Modified" both go to Plate.
	BuiltinPartType = "part_type"
	// BuiltinColor sorts by colour: one category per colour.
	BuiltinColor = "color"

	// FormatBuiltin is a built-in profile's Summary.Format.
	FormatBuiltin = "builtin"
)

// Builtins are the built-in kinds, in the order the Profiles page lists them.
var Builtins = []string{BuiltinPartType, BuiltinColor}

// BuiltinData is the stored document for a built-in kind.
func BuiltinData(kind string) []byte {
	return []byte(`{"builtin": "` + kind + `"}`)
}

// partFamilies are the usual part types, most common first: the order bins are
// pre-assigned in. Any other type still gets a category of its own when it arrives.
var partFamilies = []string{
	"Brick", "Plate", "Tile", "Slope", "Wedge", "Technic", "Bracket", "Hinge", "Arch",
	"Panel", "Cylinder", "Cone", "Bar", "Wheel", "Tire & Tread", "Wheel & Tire Assembly",
	"Window", "Door", "Windscreen", "Antenna", "Container", "Plant", "Minifigure",
}

var builtinSummaries = map[string]Summary{
	BuiltinPartType: {
		Name:        "By part type",
		Description: "Each part type gets its own bins: bricks, plates, tiles, slopes, Technic and so on, from Brickognize's category.",
	},
	BuiltinColor: {
		Name:        "By colour",
		Description: "Each colour gets its own bins, whatever the part.",
	},
}

func parseBuiltin(kind string) (*Profile, error) {
	summary, ok := builtinSummaries[kind]
	if !ok {
		return nil, invalidf("unknown built-in profile %q", kind)
	}
	summary.Builtin = kind
	summary.Format = FormatBuiltin
	summary.DefaultID = Misc
	summary.Categories = []Category{}
	if kind == BuiltinColor {
		summary.FallbackColor = true
	} else {
		for _, f := range partFamilies {
			summary.Categories = append(summary.Categories, Category{ID: f, Name: f})
		}
	}
	return &Profile{summary: summary, names: map[string]string{}, builtin: kind}, nil
}

// PartFamily is a Brickognize category's part type: the text before the first comma
// ("Technic, Liftarm" -> "Technic"). "" when there's no category.
func PartFamily(category string) string {
	family, _, _ := strings.Cut(category, ",")
	return strings.TrimSpace(family)
}

func (p *Profile) routeBuiltin(piece Piece) string {
	switch p.builtin {
	case BuiltinPartType:
		if family := PartFamily(piece.PartCategory); family != "" {
			return family
		}
	case BuiltinColor:
		if piece.Color != "" && piece.Color != AnyColor {
			return ColorCategory(piece.Color)
		}
	}
	return Misc
}

// ColorCategory is the category for a colour id, as sorting by colour names it.
func ColorCategory(color string) string {
	return colorPrefix + color
}

const colorPrefix = "color_"

// Accepts reports whether a bin may be given the category: one of the profile's,
// misc, or one it makes as pieces arrive (a colour, or any part type).
func (p *Profile) Accepts(category string) bool {
	if category == "" {
		return false
	}
	if category == Misc || p.summary.FallbackColor && strings.HasPrefix(category, colorPrefix) {
		return true
	}
	if p.builtin == BuiltinPartType {
		return true
	}
	for _, c := range p.summary.Categories {
		if c.ID == category {
			return true
		}
	}
	return false
}
