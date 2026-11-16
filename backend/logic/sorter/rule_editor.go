package sorter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"goklippy/backend/logic/sorter/profile"
)

// The rule editor (Rules › Profiles): profiles made, copied and changed on the sorter.

// SourceEditor is the source of a profile made or changed in the rule editor.
const SourceEditor = "editor"

var (
	ErrBuiltinReadOnly = errors.New("built-in profiles can't be changed; copy one and change the copy")
	// ErrProfileNeedsBins: the profile being saved is the one the bins are being sorted
	// by, so the save has to say whether the bins were emptied.
	ErrProfileNeedsBins = errors.New("the collection in the bins sorts by this profile: say whether the bins were emptied")
)

// PartKind is one part the catalog has seen: its name and type, and how many pieces.
type PartKind struct {
	PartName string `db:"part_name"`
	Category string `db:"category"`
	Seen     int    `db:"seen"`
}

// EditableProfile is a profile as the editor shows it.
type EditableProfile struct {
	Profile  StoredProfile    `json:"profile"`
	Document profile.Document `json:"document"`
	// ReadOnly: a built-in, which can only be copied.
	ReadOnly bool `json:"read_only"`
	// Converted: not made in the editor (a Hive upload); saving it turns it into an
	// editor profile in place.
	Converted bool `json:"converted"`
	// InBins: the collection in the bins sorts by it, so saving asks about the bins.
	InBins bool `json:"in_bins"`
}

// EditableProfile reads a profile as an editor document.
func (s *SortingService) EditableProfile(ctx context.Context, id int64) (EditableProfile, error) {
	stored, p, err := s.parse(ctx, id)
	if err != nil {
		return EditableProfile{}, err
	}
	_, isEditor := p.Document()
	out := EditableProfile{
		Profile: stored, Document: p.EditableDocument(),
		ReadOnly: stored.Source == SourceBuiltin, Converted: !isEditor && stored.Source != SourceBuiltin,
		InBins: s.sortsBins(id),
	}
	if out.Document.Rules == nil {
		out.Document.Rules = []profile.Rule{}
	}
	if list, err := s.store.ListProfiles(ctx); err == nil {
		for _, l := range list {
			if l.ID == id {
				out.Profile.UsedBy = l.UsedBy
			}
		}
	}
	return out, nil
}

// sortsBins reports whether the collection in the bins sorts by the profile.
func (s *SortingService) sortsBins(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collection != nil && s.active != nil && s.active.stored.ID == id
}

// CreateEditorProfile saves a new editor profile.
func (s *SortingService) CreateEditorProfile(ctx context.Context, doc profile.Document) (StoredProfile, error) {
	data, p, err := encodeDocument(doc)
	if err != nil {
		return StoredProfile{}, err
	}
	stored := StoredProfile{Name: p.Summary().Name, Source: SourceEditor, Summary: p.Summary(), UsedBy: []CollectionRef{}}
	id, err := s.store.CreateProfile(ctx, stored, data)
	if err != nil {
		return StoredProfile{}, err
	}
	stored.ID, stored.CreatedAt = id, time.Now().UTC()
	slog.Info("sorting profile made in the editor", "profile", id, "name", stored.Name, "rules", len(p.Summary().Rules))
	return stored, nil
}

// CopyProfile saves an editor copy of any profile under a new name.
func (s *SortingService) CopyProfile(ctx context.Context, id int64, name string) (StoredProfile, error) {
	_, p, err := s.parse(ctx, id)
	if err != nil {
		return StoredProfile{}, err
	}
	doc := p.EditableDocument()
	if name = strings.TrimSpace(name); name != "" {
		doc.Name = name
	} else {
		doc.Name += " (copy)"
	}
	return s.CreateEditorProfile(ctx, doc)
}

// CreateFromStarter saves a new profile from a starter.
func (s *SortingService) CreateFromStarter(ctx context.Context, key, name string) (StoredProfile, error) {
	doc, ok := profile.StarterDocument(key)
	if !ok {
		return StoredProfile{}, fmt.Errorf("%w: no starter profile %q", profile.ErrInvalid, key)
	}
	if name = strings.TrimSpace(name); name != "" {
		doc.Name = name
	}
	return s.CreateEditorProfile(ctx, doc)
}

func encodeDocument(doc profile.Document) ([]byte, *profile.Profile, error) {
	data, err := profile.DocumentJSON(doc)
	if err != nil {
		return nil, nil, err
	}
	p, err := profile.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return data, p, nil
}

// SaveEditorProfile replaces a profile with an edited document. A Hive upload becomes an
// editor profile in place (uploading the file again adds a new one). When the collection
// in the bins sorts by it, bins says what's in them: "emptied" starts the bins afresh,
// like changing the collection's profile; "continue" keeps each bin's categories that
// the profile still has. The caller makes sure no run is sorting.
func (s *SortingService) SaveEditorProfile(ctx context.Context, id int64, doc profile.Document, bins string) (StoredProfile, error) {
	stored, _, err := s.store.ProfileData(ctx, id)
	if err != nil {
		return StoredProfile{}, err
	}
	if stored.Source == SourceBuiltin {
		return StoredProfile{}, ErrBuiltinReadOnly
	}
	data, p, err := encodeDocument(doc)
	if err != nil {
		return StoredProfile{}, err
	}
	inBins := s.sortsBins(id)
	if inBins && bins != "emptied" && bins != "continue" {
		return StoredProfile{}, ErrProfileNeedsBins
	}
	stored.Name, stored.Source, stored.Summary = p.Summary().Name, SourceEditor, p.Summary()
	if err := s.store.UpdateProfile(ctx, stored, data); err != nil {
		return StoredProfile{}, err
	}
	slog.Info("sorting profile saved from the editor", "profile", id, "name", stored.Name, "rules", len(p.Summary().Rules), "bins", bins)
	if !inBins {
		return stored, nil
	}

	s.mu.Lock()
	col, counts := *s.collection, s.counts
	var kept Assignments
	if s.active != nil {
		kept = s.active.bins.Clone()
	}
	s.mu.Unlock()
	if bins == "emptied" {
		now := time.Now().UTC()
		if err := s.store.SelectCollection(ctx, col.ID, now); err != nil {
			return stored, err
		}
		if counts != nil {
			if err := counts.EmptyAllBins(ctx, now); err != nil {
				return stored, fmt.Errorf("empty the bins: %w", err)
			}
		}
		s.mu.Lock()
		s.binCounts = BinCounts{}
		s.mu.Unlock()
	} else {
		known := map[string]bool{profile.Misc: true}
		for _, c := range p.Summary().Categories {
			known[c.ID] = true
		}
		for key, cats := range kept {
			cats = slices.DeleteFunc(cats, func(c string) bool { return !known[c] && !p.Accepts(c) })
			if len(cats) == 0 {
				delete(kept, key)
			} else {
				kept[key] = cats
			}
		}
		if err := s.store.ReplaceBinCategories(ctx, col.ID, id, kept); err != nil {
			return stored, err
		}
	}
	return stored, s.load(ctx, id)
}

// CheckResult is where a piece would go under a (possibly unsaved) document.
type CheckResult struct {
	profile.Match
	CategoryName string `json:"category_name"`
	PartID       string `json:"part_id"`
	PartName     string `json:"part_name"`
	PartCategory string `json:"part_category"`
	Size         string `json:"size,omitempty"`
	// InCatalog: the part's name and type came from the catalog.
	InCatalog bool `json:"in_catalog"`
}

// CheckPiece says which rule of doc takes a part in a colour, with no kit full. The
// part's name and type come from the catalog when they aren't given.
func (s *SortingService) CheckPiece(ctx context.Context, doc profile.Document, piece profile.Piece) (CheckResult, error) {
	p, err := profile.FromDocument(doc)
	if err != nil {
		return CheckResult{}, err
	}
	piece.Part = strings.TrimSpace(piece.Part)
	out := CheckResult{PartID: piece.Part}
	if out.InCatalog, err = s.FillPartInfo(ctx, &piece); err != nil {
		return CheckResult{}, err
	}
	s.mu.Lock()
	names := s.colorNames
	s.mu.Unlock()
	out.Match = p.Match(piece, nil)
	out.CategoryName = namer(p, names)(out.Category)
	out.PartName, out.PartCategory = piece.PartName, piece.PartCategory
	out.Size = profile.PartSize(piece.PartName)
	return out, nil
}

// FillPartInfo fills in a piece's part name and type from the catalog where they're
// missing; found reports whether the catalog knew the part.
func (s *SortingService) FillPartInfo(ctx context.Context, piece *profile.Piece) (found bool, err error) {
	if piece.PartName != "" && piece.PartCategory != "" {
		return false, nil
	}
	name, category, ok, err := s.store.CatalogPartInfo(ctx, piece.Part)
	if err != nil || !ok {
		return false, err
	}
	if piece.PartName == "" {
		piece.PartName = name
	}
	if piece.PartCategory == "" {
		piece.PartCategory = category
	}
	return true, nil
}

// PickerOption is one choice in an editor picker, with how many pieces of it the
// catalog has seen.
type PickerOption struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Family string `json:"family,omitempty"`
	Seen   int    `json:"seen"`
}

// EditorOptions is what the rule editor offers.
type EditorOptions struct {
	Families []PickerOption           `json:"families"`
	Types    []PickerOption           `json:"types"`
	Sizes    []PickerOption           `json:"sizes"`
	Colors   []PickerOption           `json:"colors"`
	Presets  []profile.RulePreset     `json:"presets"`
	Starters []profile.StarterProfile `json:"starters"`
}

// EditorOptions lists the part types, sizes and colours to pick from (the usual ones and
// every one the catalog has seen), the premade rules and the starter profiles.
func (s *SortingService) EditorOptions(ctx context.Context) (EditorOptions, error) {
	kinds, err := s.store.CatalogPartKinds(ctx)
	if err != nil {
		return EditorOptions{}, err
	}
	colors, err := s.store.ColorNames(ctx)
	if err != nil {
		return EditorOptions{}, err
	}
	typeSeen, familySeen, sizeSeen := map[string]int{}, map[string]int{}, map[string]int{}
	for _, k := range kinds {
		typeSeen[k.Category] += k.Seen
		familySeen[profile.PartFamily(k.Category)] += k.Seen
		if size := profile.PartSize(k.PartName); size != "" {
			sizeSeen[size] += k.Seen
		}
	}
	out := EditorOptions{Presets: profile.RulePresets, Starters: profile.StarterProfiles}

	// Families: the usual ones first, then any other seen or listed, by name.
	families := profile.PartFamilies()
	var more []string
	for _, t := range profile.CommonPartTypes {
		more = append(more, profile.PartFamily(t))
	}
	for f := range familySeen {
		more = append(more, f)
	}
	sort.Strings(more)
	for _, f := range more {
		if f != "" && !slices.Contains(families, f) {
			families = append(families, f)
		}
	}
	for _, f := range families {
		out.Families = append(out.Families, PickerOption{Value: f, Label: f, Seen: familySeen[f]})
	}

	// Types: grouped by family in the families' order; the plain one first in each.
	types := append([]string(nil), profile.CommonPartTypes...)
	for t := range typeSeen {
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	familyOrder := map[string]int{}
	for i, f := range families {
		familyOrder[f] = i
	}
	sort.SliceStable(types, func(i, j int) bool {
		fi, fj := familyOrder[profile.PartFamily(types[i])], familyOrder[profile.PartFamily(types[j])]
		if fi != fj {
			return fi < fj
		}
		pi, pj := types[i] == profile.PartFamily(types[i]), types[j] == profile.PartFamily(types[j])
		if pi != pj {
			return pi
		}
		return types[i] < types[j]
	})
	for _, t := range types {
		label := t
		if t == profile.PartFamily(t) {
			label = t + " (standard)"
		}
		out.Types = append(out.Types, PickerOption{Value: t, Label: label, Family: profile.PartFamily(t), Seen: typeSeen[t]})
	}

	// Sizes: the common ones, then any other seen, smallest first.
	sizes := append([]string(nil), profile.CommonSizes...)
	var seenSizes []string
	for sz := range sizeSeen {
		if !slices.Contains(sizes, sz) {
			seenSizes = append(seenSizes, sz)
		}
	}
	sort.Strings(seenSizes)
	for _, sz := range append(sizes, seenSizes...) {
		out.Sizes = append(out.Sizes, PickerOption{Value: sz, Label: profile.SizeLabel(sz), Seen: sizeSeen[sz]})
	}

	all := map[string]string{}
	for id, name := range profile.CommonColors {
		all[id] = name
	}
	for id, name := range colors {
		if name != "" {
			all[id] = name
		}
	}
	for id, name := range all {
		out.Colors = append(out.Colors, PickerOption{Value: id, Label: name})
	}
	sort.Slice(out.Colors, func(i, j int) bool { return out.Colors[i].Label < out.Colors[j].Label })
	if out.Colors == nil {
		out.Colors = []PickerOption{}
	}
	return out, nil
}
