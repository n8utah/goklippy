package logic

import "context"

// SetInventory is one set and the parts it takes, for the sorter's set rules: a set
// rule pulls these parts out into the set's own bins.
type SetInventory struct {
	SetNum string          `json:"set_num"`
	Name   string          `json:"name"`
	Year   int             `json:"year"`
	ImgURL string          `json:"img_url"`
	Parts  []InventoryLine `json:"parts"`
}

// InventoryLine is how many of a part, in a colour, a set takes. Part numbers and
// colour names are Rebrickable's; the sorter maps colours to its own by name.
type InventoryLine struct {
	PartNum   string `json:"part_num"`
	ColorName string `json:"color_name"`
	Need      int    `json:"need"`
}

// SetInventory is a set's parts (no spares or stickers), for a sorter.
func (s *Service) SetInventory(ctx context.Context, setNum string) (SetInventory, error) {
	var out SetInventory
	err := s.withCatalog(func(cat SetCatalog) error {
		set, lines, err := cat.SetParts(ctx, setNum, nil)
		if err != nil {
			return err
		}
		out = SetInventory{SetNum: set.SetNum, Name: set.Name, Year: set.Year, ImgURL: set.ImgURL,
			Parts: make([]InventoryLine, 0, len(lines))}
		for _, l := range lines {
			out.Parts = append(out.Parts, InventoryLine{PartNum: l.PartNum, ColorName: l.ColorName, Need: l.Need})
		}
		return nil
	})
	return out, err
}
