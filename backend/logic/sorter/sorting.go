package sorter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"goklippy/backend/logic/sorter/profile"
)

var (
	ErrProfileNotFound   = errors.New("sorting profile not found")
	ErrNoActiveProfile   = errors.New("no sorting profile is active; activate one on the Profiles page")
	ErrProfileActive     = errors.New("that profile is active; activate another before deleting it")
	ErrUnknownCategory   = errors.New("not a category of the active profile")
	ErrInvalidActivation = errors.New("activation mode must be keep, reset or preassign")
)

// StoredProfile is a saved profile without its contents.
type StoredProfile struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Source    string          `json:"source"`
	Active    bool            `json:"active"`
	CreatedAt time.Time       `json:"created_at"`
	Summary   profile.Summary `json:"summary"`
}

// SortingStore keeps profiles, the current collection, and each collection's bin
// assignments under each profile.
type SortingStore interface {
	ListProfiles(ctx context.Context) ([]StoredProfile, error)
	// ProfileData returns ErrProfileNotFound for an unknown id.
	ProfileData(ctx context.Context, id int64) (StoredProfile, []byte, error)
	CreateProfile(ctx context.Context, p StoredProfile, data []byte) (int64, error)
	DeleteProfile(ctx context.Context, id int64) error
	ActivateProfile(ctx context.Context, id int64) error
	ActiveProfileID(ctx context.Context) (int64, bool, error)
	CurrentCollection(ctx context.Context) (CollectionRef, bool, error)
	// SelectCollection makes the collection current and clears its bin assignments
	// under every profile, as of at.
	SelectCollection(ctx context.Context, id int64, at time.Time) error
	BinCategories(ctx context.Context, collectionID, profileID int64) (Assignments, error)
	ReplaceBinCategories(ctx context.Context, collectionID, profileID int64, bins Assignments) error
	SetBinCategories(ctx context.Context, collectionID, profileID int64, bin BinKey, categories []string) error
}

// Activation says what happens to the bins when a profile is activated.
type Activation string

const (
	ActivateKeep      Activation = "keep"      // keep the bins it had last time
	ActivateReset     Activation = "reset"     // every bin starts unassigned
	ActivatePreassign Activation = "preassign" // rule order, from layer 1 bin 1
)

// SortingService answers where pieces go under the active profile and keeps the
// bins' categories.
type SortingService struct {
	store    SortingStore
	settings func() MachineSettings

	mu     sync.Mutex
	active *activeProfile
	// collection is the current collection: whose bins these are. nil until one is picked.
	collection *CollectionRef
	rng        *rand.Rand
	// What's in each bin, when AttachCounts was called.
	counts    CountsStore
	binCounts BinCounts
}

type activeProfile struct {
	stored  StoredProfile
	profile *profile.Profile
	bins    Assignments
	kits    *kitTracker
}

// CountsStore answers how full bins and kits are, from the piece records.
type CountsStore interface {
	// BinCounts is the pieces delivered to each bin since it was last emptied.
	BinCounts(ctx context.Context) (BinCounts, error)
	EmptyBin(ctx context.Context, bin BinKey, at time.Time) error
	// EmptyAllBins records every bin emptied at at.
	EmptyAllBins(ctx context.Context, at time.Time) error
	// KitPieces lists the pieces delivered into a profile's kit categories for the
	// collection since its bins last started afresh, oldest first, to count kits up
	// again after a restart.
	KitPieces(ctx context.Context, collectionID, profileID int64, categories []string) ([]KitPiece, error)
}

// KitPiece is one delivered piece a kit took.
type KitPiece struct {
	Category string `db:"category"`
	Part     string `db:"part_id"`
	Color    string `db:"color_id"`
}

// AttachCounts loads bin and kit counts, so full bins and full kits take no more.
func (s *SortingService) AttachCounts(ctx context.Context, counts CountsStore) error {
	bins, err := counts.BinCounts(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.counts, s.binCounts = counts, bins
	active := s.active
	s.mu.Unlock()
	if active != nil {
		return s.load(ctx, active.stored.ID)
	}
	return nil
}

// NoteDelivered counts a piece that dropped into a bin.
func (s *SortingService) NoteDelivered(bin BinKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binCounts == nil {
		s.binCounts = BinCounts{}
	}
	s.binCounts[bin]++
}

// EmptyBin records that a bin was emptied: it counts from zero again.
func (s *SortingService) EmptyBin(ctx context.Context, bin BinKey) error {
	s.mu.Lock()
	counts := s.counts
	s.mu.Unlock()
	if counts != nil {
		if err := counts.EmptyBin(ctx, bin, time.Now().UTC()); err != nil {
			return err
		}
	}
	s.mu.Lock()
	delete(s.binCounts, bin)
	s.mu.Unlock()
	return nil
}

// KitProgress is how far each kit of the active profile has got.
func (s *SortingService) KitProgress() []KitProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return []KitProgress{}
	}
	return s.active.kits.progress(s.active.profile.CategoryName)
}

// NewSortingService loads the active profile, if any. A profile that can't be read
// is logged and left inactive, so the backend still starts.
func NewSortingService(ctx context.Context, store SortingStore, settings func() MachineSettings) *SortingService {
	s := &SortingService{store: store, settings: settings, rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))}
	if col, ok, err := store.CurrentCollection(ctx); err != nil {
		slog.Warn("current collection not loaded", "error", err)
	} else if ok {
		s.collection = &col
	}
	id, ok, err := store.ActiveProfileID(ctx)
	if err != nil {
		slog.Warn("sorting profiles not loaded", "error", err)
		return s
	}
	if ok {
		if err := s.load(ctx, id); err != nil {
			slog.Warn("active sorting profile not loaded", "id", id, "error", err)
		}
	}
	return s
}

// parse reads a saved profile.
func (s *SortingService) parse(ctx context.Context, id int64) (StoredProfile, *profile.Profile, error) {
	stored, data, err := s.store.ProfileData(ctx, id)
	if err != nil {
		return StoredProfile{}, nil, err
	}
	p, err := profile.Parse(data)
	if err != nil {
		return StoredProfile{}, nil, err
	}
	return stored, p, nil
}

// load makes a profile the active one, with the current collection's bins under it.
func (s *SortingService) load(ctx context.Context, id int64) error {
	stored, p, err := s.parse(ctx, id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	counts, col := s.counts, s.collection
	s.mu.Unlock()
	bins := Assignments{}
	if col != nil {
		if bins, err = s.store.BinCategories(ctx, col.ID, id); err != nil {
			return err
		}
	}
	kits := newKitTracker(p.Kits())
	if counts != nil && col != nil && len(p.Kits()) > 0 {
		categories := make([]string, 0, len(p.Kits()))
		for c := range p.Kits() {
			categories = append(categories, c)
		}
		taken, err := counts.KitPieces(ctx, col.ID, id, categories)
		if err != nil {
			return err
		}
		for _, k := range taken {
			kits.record(k.Category, k.Part, k.Color)
		}
	}
	stored.Active = true
	s.mu.Lock()
	s.active = &activeProfile{stored: stored, profile: p, bins: bins, kits: kits}
	s.mu.Unlock()
	return nil
}

// Active is the active profile, or nil.
func (s *SortingService) Active() *StoredProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil
	}
	p := s.active.stored
	return &p
}

// Profiles lists the saved profiles.
func (s *SortingService) Profiles(ctx context.Context) ([]StoredProfile, error) {
	return s.store.ListProfiles(ctx)
}

// Import saves an uploaded profile file. It doesn't activate it.
func (s *SortingService) Import(ctx context.Context, data []byte) (StoredProfile, error) {
	p, err := profile.Parse(data)
	if err != nil {
		return StoredProfile{}, err
	}
	stored := StoredProfile{Name: p.Summary().Name, Source: "upload", Summary: p.Summary()}
	id, err := s.store.CreateProfile(ctx, stored, data)
	if err != nil {
		return StoredProfile{}, err
	}
	stored.ID = id
	stored.CreatedAt = time.Now().UTC()
	return stored, nil
}

// Delete removes a profile that isn't active, with its bin assignments.
func (s *SortingService) Delete(ctx context.Context, id int64) error {
	s.mu.Lock()
	active := s.active != nil && s.active.stored.ID == id
	s.mu.Unlock()
	if active {
		return ErrProfileActive
	}
	return s.store.DeleteProfile(ctx, id)
}

// Activate makes a profile the one the machine sorts with.
func (s *SortingService) Activate(ctx context.Context, id int64, mode Activation) error {
	switch mode {
	case ActivateKeep, ActivateReset, ActivatePreassign:
	default:
		return ErrInvalidActivation
	}
	_, p, err := s.parse(ctx, id)
	if err != nil {
		return err
	}
	col := s.Collection()
	if col == nil && mode != ActivateKeep {
		return ErrNoCollection // the bins being reset or preassigned are a collection's
	}
	if err := s.store.ActivateProfile(ctx, id); err != nil {
		return err
	}
	switch mode {
	case ActivateReset:
		err = s.store.ReplaceBinCategories(ctx, col.ID, id, Assignments{})
	case ActivatePreassign:
		err = s.store.ReplaceBinCategories(ctx, col.ID, id, s.preassign(p))
	}
	if err != nil {
		return err
	}
	return s.load(ctx, id)
}

// preassign gives each category of the profile, in rule order, the next usable bin.
func (s *SortingService) preassign(p *profile.Profile) Assignments {
	bins := Assignments{}
	usable := s.settings().Distributor.usableBins(0, nil)
	for i, c := range p.Summary().Categories {
		if i >= len(usable) {
			break
		}
		bins[BinKey{usable[i].Layer, usable[i].Bin}] = []string{c.ID}
	}
	return bins
}

// CategoryRef is a category with its name, for the UI.
type CategoryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BinView is one bin on the Bins page.
type BinView struct {
	BinPosition
	Categories []CategoryRef `json:"categories"`
	Count      int           `json:"count"` // pieces since it was last emptied
	Full       bool          `json:"full"`
}

type SectionView struct {
	Section  int       `json:"section"`
	Disabled bool      `json:"disabled"`
	Bins     []BinView `json:"bins"`
}

type LayerView struct {
	Layer      int           `json:"layer"`
	Bins       int           `json:"bins"`
	Disabled   bool          `json:"disabled"`
	MaxPieceMm float64       `json:"max_piece_mm"`
	MaxPieces  int           `json:"max_pieces"`
	Sections   []SectionView `json:"sections"`
}

// BinsView is the tower with each bin's categories: the current collection's bins
// under the active profile.
type BinsView struct {
	Collection *CollectionRef `json:"collection"`
	Profile    *StoredProfile `json:"profile"`
	// Categories are the ones a bin can be given: the profile's, then misc.
	Categories []CategoryRef `json:"categories"`
	SharedBins bool          `json:"shared_bins"`
	Layers     []LayerView   `json:"layers"`
}

// Bins describes every bin and what it takes.
func (s *SortingService) Bins() BinsView {
	d := s.settings().Distributor
	s.mu.Lock()
	active := s.active
	var assigned Assignments
	if active != nil {
		assigned = active.bins.Clone() // Route may add to the live map
	}
	counts := BinCounts{}
	for k, v := range s.binCounts {
		counts[k] = v
	}
	s.mu.Unlock()

	view := BinsView{Collection: s.Collection(), SharedBins: d.SharedBins, Categories: []CategoryRef{}, Layers: []LayerView{}}
	if active != nil {
		stored := active.stored
		view.Profile = &stored
		listed := map[string]bool{profile.Misc: true}
		for _, c := range active.profile.Summary().Categories {
			view.Categories = append(view.Categories, CategoryRef{c.ID, c.Name})
			listed[c.ID] = true
		}
		// Categories made as pieces arrive (color_7) are listed once a bin has one.
		var extra []string
		for _, cats := range assigned {
			for _, c := range cats {
				if !listed[c] {
					listed[c] = true
					extra = append(extra, c)
				}
			}
		}
		slices.Sort(extra)
		for _, c := range extra {
			view.Categories = append(view.Categories, CategoryRef{c, active.profile.CategoryName(c)})
		}
		view.Categories = append(view.Categories, CategoryRef{profile.Misc, active.profile.CategoryName(profile.Misc)})
	}
	for l, layerBins := range d.LayerBins() {
		layer := d.Layers[l]
		lv := LayerView{Layer: l + 1, Bins: layer.Bins, Disabled: layer.Disabled, MaxPieceMm: layer.LayerMaxPieceMm(), MaxPieces: layer.MaxPieces, Sections: []SectionView{}}
		for _, pos := range layerBins {
			if len(lv.Sections) < pos.Section {
				lv.Sections = append(lv.Sections, SectionView{
					Section: pos.Section, Disabled: slices.Contains(layer.DisabledSections, pos.Section), Bins: []BinView{},
				})
			}
			key := BinKey{pos.Layer, pos.Bin}
			bv := BinView{BinPosition: pos, Categories: []CategoryRef{}, Count: counts[key]}
			bv.Full = layer.MaxPieces > 0 && bv.Count >= layer.MaxPieces
			if active != nil {
				for _, id := range assigned[BinKey{pos.Layer, pos.Bin}] {
					bv.Categories = append(bv.Categories, CategoryRef{id, active.profile.CategoryName(id)})
				}
			}
			sec := &lv.Sections[pos.Section-1]
			sec.Bins = append(sec.Bins, bv)
		}
		view.Layers = append(view.Layers, lv)
	}
	return view
}

// SetBin gives a bin exactly these categories. A category that was in another bin
// moves here.
func (s *SortingService) SetBin(ctx context.Context, key BinKey, categories []string) error {
	if _, err := s.settings().Distributor.BinPosition(BinAddress{Layer: key.Layer, Bin: key.Bin}); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ErrNoActiveProfile
	}
	if s.collection == nil {
		return ErrNoCollection
	}
	known := s.knownCategoriesLocked()
	byColor := s.active.profile.Summary().FallbackColor
	var cats []string
	for _, c := range categories {
		if !known[c] && !(byColor && strings.HasPrefix(c, "color_")) {
			return fmt.Errorf("%w: %q", ErrUnknownCategory, c)
		}
		if !slices.Contains(cats, c) {
			cats = append(cats, c)
		}
	}

	bins := s.active.bins.Clone()
	for k, v := range bins {
		bins[k] = slices.DeleteFunc(v, func(c string) bool { return slices.Contains(cats, c) })
		if len(bins[k]) == 0 {
			delete(bins, k)
		}
	}
	if len(cats) > 0 {
		bins[key] = cats
	} else {
		delete(bins, key)
	}
	if err := s.store.ReplaceBinCategories(ctx, s.collection.ID, s.active.stored.ID, bins); err != nil {
		return err
	}
	s.active.bins = bins
	return nil
}

// knownCategoriesLocked is every category a bin may take: the profile's, misc, and
// any a bin already has (such as color_7, made as pieces arrive when the profile
// sorts the rest by color). s.mu must be held.
func (s *SortingService) knownCategoriesLocked() map[string]bool {
	known := map[string]bool{profile.Misc: true}
	for _, c := range s.active.profile.Summary().Categories {
		known[c.ID] = true
	}
	for _, cats := range s.active.bins {
		for _, c := range cats {
			known[c] = true
		}
	}
	return known
}

// ResetBins unassigns every bin of the current collection under the active profile.
func (s *SortingService) ResetBins(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ErrNoActiveProfile
	}
	if s.collection == nil {
		return ErrNoCollection
	}
	if err := s.store.ReplaceBinCategories(ctx, s.collection.ID, s.active.stored.ID, Assignments{}); err != nil {
		return err
	}
	s.active.bins = Assignments{}
	return nil
}

// RouteResult is where a piece goes and why.
type RouteResult struct {
	Category     string    `json:"category"`
	CategoryName string    `json:"category_name"`
	Choice       BinChoice `json:"choice"`
}

// Route decides where a piece goes: its category under the active profile, then
// its bin. pieceMm is its longest side (0 if unknown). With commit, a bin newly
// given the category is saved; without, nothing changes (for "where would this
// go?").
func (s *SortingService) Route(ctx context.Context, part, color string, pieceMm float64, commit bool) (RouteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return RouteResult{}, ErrNoActiveProfile
	}
	kits := s.active.kits
	category := s.active.profile.Route(part, color, kits.full)
	res, err := s.routeLocked(ctx, category, pieceMm, commit)
	if err == nil && commit {
		kits.record(category, part, color)
	}
	return res, err
}

// RouteCategory finds the bin for a category directly, such as misc for a piece that
// couldn't be identified (so a bin given misc by hand catches it).
func (s *SortingService) RouteCategory(ctx context.Context, category string, pieceMm float64, commit bool) (RouteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return RouteResult{}, ErrNoActiveProfile
	}
	return s.routeLocked(ctx, category, pieceMm, commit)
}

func (s *SortingService) routeLocked(ctx context.Context, category string, pieceMm float64, commit bool) (RouteResult, error) {
	d := s.settings().Distributor
	choice, newCats := d.ChooseBin(s.active.bins, s.binCounts, category, pieceMm, s.rng.IntN)
	result := RouteResult{Category: category, CategoryName: s.active.profile.CategoryName(category), Choice: choice}
	if !commit || !choice.Assigned {
		return result, nil
	}
	if s.collection == nil {
		return RouteResult{}, ErrNoCollection
	}
	key := BinKey{choice.Bin.Layer, choice.Bin.Bin}
	if err := s.store.SetBinCategories(ctx, s.collection.ID, s.active.stored.ID, key, newCats); err != nil {
		return RouteResult{}, fmt.Errorf("save the bin for %s: %w", category, err)
	}
	s.active.bins[key] = newCats
	slog.Info("bin assigned", "category", category, "layer", key.Layer, "bin", key.Bin, "categories", newCats)
	return result, nil
}
