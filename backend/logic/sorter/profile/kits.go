package profile

import (
	"encoding/json"
	"sort"
)

// AnyColorID is how a set inventory says "any color".
const AnyColorID = "-1"

// KitLine is how many of a part, in a color ("" for any), a kit needs.
type KitLine struct {
	Part  string `json:"part"`
	Color string `json:"color,omitempty"`
	Need  int    `json:"need"`
}

type inventory struct {
	Name  string          `json:"name"`
	Parts []inventoryPart `json:"parts"`
}

type inventoryPart struct {
	PartNum  string  `json:"part_num"`
	PartID   string  `json:"part_id"` // older profiles
	ColorID  colorID `json:"color_id"`
	Quantity int     `json:"quantity"`
}

// parseKits reads set_inventories: per kit category, its lines, merged per part and
// color. An inventory stored as a bare list (older profiles) belongs to "set_<key>".
func parseKits(raw map[string]json.RawMessage) (map[string][]KitLine, map[string]string) {
	kits := map[string][]KitLine{}
	names := map[string]string{}
	for key, data := range raw {
		category := key
		var inv inventory
		if err := json.Unmarshal(data, &inv); err != nil {
			var parts []inventoryPart
			if json.Unmarshal(data, &parts) != nil {
				continue
			}
			inv.Parts = parts
			if len(category) < 4 || category[:4] != "set_" {
				category = "set_" + category
			}
		}
		merged := map[[2]string]int{}
		for _, p := range inv.Parts {
			part := p.PartNum
			if part == "" {
				part = p.PartID
			}
			color := string(p.ColorID)
			if color == AnyColorID {
				color = ""
			}
			if part != "" && p.Quantity > 0 {
				merged[[2]string{part, color}] += p.Quantity
			}
		}
		lines := make([]KitLine, 0, len(merged))
		for k, need := range merged {
			lines = append(lines, KitLine{Part: k[0], Color: k[1], Need: need})
		}
		sort.Slice(lines, func(i, j int) bool {
			if lines[i].Part != lines[j].Part {
				return lines[i].Part < lines[j].Part
			}
			return lines[i].Color < lines[j].Color
		})
		kits[category] = lines
		if inv.Name != "" {
			names[category] = inv.Name
		}
	}
	return kits, names
}

// Kits is each kit category's inventory; empty for a profile without set inventories.
func (p *Profile) Kits() map[string][]KitLine {
	return p.kits
}
