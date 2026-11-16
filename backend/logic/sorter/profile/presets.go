package profile

// Premade rules and starter profiles for the editor: a rule is added with one click and
// then changed; a starter is copied into a new profile. Part types are BrickLink's
// category names, which is what Brickognize reports.

// RulePreset is a ready-made rule.
type RulePreset struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Rule  Rule   `json:"rule"`
}

// StarterProfile is a ready-made profile to start from.
type StarterProfile struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rules       []string `json:"rules"` // preset keys, in order
	Leftover    string   `json:"leftover"`
}

// RulePresets are the premade rules, in the order the editor offers them.
var RulePresets = []RulePreset{
	{Key: "standard_bricks", Group: "Basics", Rule: Rule{
		Name: "Bricks", Description: "Standard bricks, a bin per size.",
		Types: []string{"Brick"}, Split: SplitSize,
	}},
	{Key: "standard_plates", Group: "Basics", Rule: Rule{
		Name: "Plates", Description: "Standard plates, a bin per size.",
		Types: []string{"Plate"}, Split: SplitSize,
	}},
	{Key: "tiles", Group: "Basics", Rule: Rule{
		Name: "Tiles", Description: "Every tile: plain, round and modified.",
		Families: []string{"Tile"},
	}},
	{Key: "slopes", Group: "Basics", Rule: Rule{
		Name: "Slopes", Description: "Every slope: plain, curved and inverted.",
		Families: []string{"Slope"},
	}},
	{Key: "wedges", Group: "Basics", Rule: Rule{
		Name: "Wedges", Description: "Wedge bricks and wedge plates.",
		Families: []string{"Wedge"},
	}},
	{Key: "round", Group: "Basics", Rule: Rule{
		Name: "Round bricks & plates", Description: "Round bricks, plates and tiles.",
		Types: []string{"Brick, Round", "Plate, Round", "Tile, Round"},
	}},
	{Key: "modified_bricks", Group: "Basics", Rule: Rule{
		Name: "Modified bricks", Description: "Bricks with clips, studs on the side, grilles and so on.",
		Types: []string{"Brick, Modified"},
	}},
	{Key: "modified_plates", Group: "Basics", Rule: Rule{
		Name: "Modified plates", Description: "Plates with clips, bars, handles, studs on the side and so on.",
		Types: []string{"Plate, Modified"},
	}},
	{Key: "brackets_hinges", Group: "Basics", Rule: Rule{
		Name: "Brackets & hinges", Description: "Brackets and every kind of hinge.",
		Families: []string{"Bracket", "Hinge"},
	}},
	{Key: "technic_beams", Group: "Technic", Rule: Rule{
		Name: "Technic beams", Description: "Liftarms and Technic bricks.",
		Types: []string{"Technic, Liftarm", "Technic, Brick"},
	}},
	{Key: "technic_pins", Group: "Technic", Rule: Rule{
		Name: "Technic pins & axles", Description: "Pins, axles, bushes and connectors.",
		Types: []string{"Technic, Pin", "Technic, Axle", "Technic, Bush", "Technic, Connector"},
	}},
	{Key: "technic_gears", Group: "Technic", Rule: Rule{
		Name: "Technic gears", Description: "Gears and gear racks.",
		Types: []string{"Technic, Gear"},
	}},
	{Key: "wheels", Group: "Other", Rule: Rule{
		Name: "Wheels & tyres", Description: "Wheels, tyres and wheels with tyres on.",
		Families: []string{"Wheel", "Tire & Tread", "Wheel & Tire Assembly"},
	}},
	{Key: "windows_doors", Group: "Other", Rule: Rule{
		Name: "Windows & doors", Description: "Windows, doors, their frames and windscreens.",
		Families: []string{"Window", "Door", "Windscreen"},
	}},
	{Key: "minifigures", Group: "Other", Rule: Rule{
		Name: "Minifigure parts", Description: "Heads, torsos, legs, hair, hats and their tools.",
		Families: []string{"Minifigure", "Mini Doll"},
	}},
	{Key: "plants_animals", Group: "Other", Rule: Rule{
		Name: "Plants & animals", Description: "Leaves, flowers, trees and animals.",
		Families: []string{"Plant", "Animal"},
	}},
}

// StarterProfiles are the premade profiles to start from.
var StarterProfiles = []StarterProfile{
	{
		Key:         "sizes_then_types",
		Name:        "Bricks and plates by size, the rest by part type",
		Description: "Standard bricks and plates get a bin per size; everything else a bin per part type.",
		Rules:       []string{"standard_bricks", "standard_plates"},
		Leftover:    LeftoverPartType,
	},
	{
		Key:         "basics_then_colour",
		Name:        "Building basics, the rest by colour",
		Description: "Bricks and plates by size, tiles, slopes and the common Technic parts each in their own bins; everything else by colour.",
		Rules:       []string{"standard_bricks", "standard_plates", "tiles", "slopes", "technic_beams", "technic_pins"},
		Leftover:    LeftoverColor,
	},
}

// RulePresetByKey finds a premade rule.
func RulePresetByKey(key string) (RulePreset, bool) {
	for _, p := range RulePresets {
		if p.Key == key {
			return p, true
		}
	}
	return RulePreset{}, false
}

// StarterDocument is a starter profile as a document to save.
func StarterDocument(key string) (Document, bool) {
	for _, s := range StarterProfiles {
		if s.Key != key {
			continue
		}
		doc := Document{Name: s.Name, Description: s.Description, Leftover: s.Leftover}
		for _, k := range s.Rules {
			if p, ok := RulePresetByKey(k); ok {
				doc.Rules = append(doc.Rules, p.Rule)
			}
		}
		return doc, true
	}
	return Document{}, false
}

// PartFamilies are the usual part type families, most common first.
func PartFamilies() []string {
	return append([]string(nil), partFamilies...)
}

// CommonPartTypes are the usual exact part types, offered before the catalog has seen
// them, grouped by family in the order of PartFamilies.
var CommonPartTypes = []string{
	"Brick", "Brick, Modified", "Brick, Round", "Brick, Decorated",
	"Plate", "Plate, Modified", "Plate, Round",
	"Tile", "Tile, Modified", "Tile, Round", "Tile, Decorated",
	"Slope", "Slope, Curved", "Slope, Inverted", "Slope, Curved, Inverted",
	"Wedge", "Wedge, Plate",
	"Technic", "Technic, Liftarm", "Technic, Brick", "Technic, Pin", "Technic, Axle",
	"Technic, Bush", "Technic, Connector", "Technic, Gear", "Technic, Panel", "Technic, Plate",
	"Bracket", "Hinge", "Arch", "Panel", "Cylinder", "Cone", "Bar", "Dish",
	"Wheel", "Tire & Tread", "Wheel & Tire Assembly",
	"Window", "Door", "Windscreen", "Antenna", "Container", "Fence", "Plant",
	"Minifigure, Body Part", "Minifigure, Hair", "Minifigure, Headgear", "Minifigure, Utensil",
	"Minifigure, Weapon", "Animal, Land",
}

// CommonColors are the usual BrickLink colours (id -> name), offered before the sorter
// has identified them.
var CommonColors = map[string]string{
	"1": "White", "2": "Tan", "3": "Yellow", "4": "Orange", "5": "Red", "6": "Green", "7": "Blue",
	"8": "Brown", "11": "Black", "12": "Trans-Clear", "17": "Trans-Red", "23": "Pink", "24": "Purple",
	"34": "Lime", "36": "Bright Green", "42": "Medium Blue", "59": "Dark Red", "63": "Dark Blue",
	"68": "Dark Orange", "69": "Dark Tan", "80": "Dark Green", "85": "Dark Bluish Gray",
	"86": "Light Bluish Gray", "88": "Reddish Brown", "103": "Bright Light Yellow",
	"110": "Bright Light Orange", "120": "Dark Brown", "153": "Dark Azure", "156": "Medium Azure",
	"9": "Light Gray", "10": "Dark Gray", "14": "Trans-Dark Blue", "15": "Trans-Light Blue",
	"19": "Trans-Yellow", "20": "Trans-Green", "28": "Nougat", "39": "Dark Turquoise", "47": "Dark Pink",
	"48": "Sand Green", "55": "Sand Blue", "71": "Magenta", "89": "Dark Purple", "95": "Flat Silver",
	"98": "Trans-Orange", "104": "Bright Pink", "115": "Pearl Gold", "150": "Medium Nougat",
	"152": "Light Aqua", "154": "Lavender", "155": "Olive Green", "157": "Medium Lavender",
	"158": "Yellowish Green",
}
