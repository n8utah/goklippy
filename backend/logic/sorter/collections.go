package sorter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Collections: one person's LEGO being sorted, over as many runs as it takes. Every run
// sorts into one collection; its pieces, counts and unknowns add up across its runs.
// One collection is current: the one the bins hold and the next run sorts into.
// Switching to another means the bins were emptied, so its bin layout starts afresh.

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrCollectionName     = errors.New("a collection needs a name no other collection has")
	ErrNoCollection       = errors.New("no collection is picked; pick the collection to sort into")
	ErrCollectionArchived = errors.New("that collection is archived; unarchive it first")
	ErrCollectionSorting  = errors.New("that collection is being sorted; stop the run first")
)

// Collection is a collection with its totals.
type Collection struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Current bool   `json:"current"`
	// BinsStartedAt is when it last became current: its bins started afresh then.
	BinsStartedAt *time.Time `json:"bins_started_at,omitempty"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	// ProfileID is the profile it sorts by; nil only for collections made before
	// collections had profiles.
	ProfileID   *int64 `json:"profile_id"`
	ProfileName string `json:"profile_name"`

	Runs       int `json:"runs"`
	Pieces     int `json:"pieces"`
	Identified int `json:"identified"`
	// Unknown is the pieces with no part: unidentified, failed to identify, or lost
	// before they were identified (pieces still on their way through aren't counted).
	Unknown      int        `json:"unknown"`
	LastSortedAt *time.Time `json:"last_sorted_at,omitempty"`
}

// CollectionRef names a collection, for the machine's snapshot.
type CollectionRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ProfileID *int64 `json:"profile_id,omitempty"`
}

// CollectionStore keeps collections. The records store implements it.
type CollectionStore interface {
	ListCollections(ctx context.Context) ([]Collection, error)
	// GetCollection returns ErrCollectionNotFound for an unknown id.
	GetCollection(ctx context.Context, id int64) (Collection, error)
	// CreateCollection and RenameCollection return ErrCollectionName for a name in use.
	CreateCollection(ctx context.Context, name string, profileID *int64, at time.Time) (int64, error)
	RenameCollection(ctx context.Context, id int64, name string) error
	// ArchiveCollection archives (at set) or unarchives (at nil) a collection.
	ArchiveCollection(ctx context.Context, id int64, at *time.Time) error
	// CollectionBins is the pieces delivered to each bin since the collection's bins
	// last started afresh.
	CollectionBins(ctx context.Context, id int64) (BinCounts, error)
	// ClearUnknowns deletes the collection's unknown pieces and returns their ids, so
	// their photos can be removed.
	ClearUnknowns(ctx context.Context, id int64) ([]string, error)
}

// Collections answers the collection pages.
type Collections struct {
	store   CollectionStore
	sorting *SortingService
	dataDir string
	// Sorting reports the collection being sorted now, if any.
	sortingNow func() int64
}

// NewCollections serves collections from records' store. sortingNow is the id of the
// collection a run is sorting into now (0 when none).
func NewCollections(records *Records, sorting *SortingService, sortingNow func() int64) (*Collections, error) {
	store, ok := records.Store().(CollectionStore)
	if !ok {
		return nil, errors.New("the records store doesn't keep collections")
	}
	return &Collections{store: store, sorting: sorting, dataDir: records.DataDir(), sortingNow: sortingNow}, nil
}

func (c *Collections) List(ctx context.Context) ([]Collection, error) {
	return c.store.ListCollections(ctx)
}

func (c *Collections) Get(ctx context.Context, id int64) (Collection, error) {
	return c.store.GetCollection(ctx, id)
}

// Create makes a collection that sorts by the profile.
func (c *Collections) Create(ctx context.Context, name string, profileID int64) (Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Collection{}, ErrCollectionName
	}
	if profileID == 0 {
		return Collection{}, ErrNoProfile
	}
	if err := c.sorting.checkProfile(ctx, profileID); err != nil {
		return Collection{}, err
	}
	id, err := c.store.CreateCollection(ctx, name, &profileID, time.Now().UTC())
	if err != nil {
		return Collection{}, err
	}
	return c.store.GetCollection(ctx, id)
}

func (c *Collections) Rename(ctx context.Context, id int64, name string) (Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Collection{}, ErrCollectionName
	}
	if err := c.store.RenameCollection(ctx, id, name); err != nil {
		return Collection{}, err
	}
	c.sorting.renamedCollection(id, name)
	return c.store.GetCollection(ctx, id)
}

// Archive hides a finished collection from the pickers; it can't be sorted into until
// it's unarchived. The current collection stays current.
func (c *Collections) Archive(ctx context.Context, id int64, archived bool) (Collection, error) {
	var at *time.Time
	if archived {
		if c.sortingNow() == id {
			return Collection{}, ErrCollectionSorting
		}
		now := time.Now().UTC()
		at = &now
	}
	if err := c.store.ArchiveCollection(ctx, id, at); err != nil {
		return Collection{}, err
	}
	return c.store.GetCollection(ctx, id)
}

// ClearUnknowns deletes the collection's unknown pieces and their photos, before the
// unknowns are fed through again, so they aren't counted twice.
func (c *Collections) ClearUnknowns(ctx context.Context, id int64) (int, error) {
	if c.sortingNow() == id {
		return 0, ErrCollectionSorting
	}
	if _, err := c.store.GetCollection(ctx, id); err != nil {
		return 0, err
	}
	ids, err := c.store.ClearUnknowns(ctx, id)
	if err != nil {
		return 0, err
	}
	for _, pid := range ids {
		if len(pid) < 2 {
			continue
		}
		dir := filepath.Join(c.dataDir, "pieces", pid[:2], pid)
		if err := os.RemoveAll(dir); err != nil {
			slog.Warn("removing an unknown piece's photos failed", "piece", pid, "error", err)
		}
	}
	slog.Info("collection unknowns cleared", "collection", id, "pieces", len(ids))
	return len(ids), nil
}

// PlanBin is one bin of a collection's sorting plan.
type PlanBin struct {
	Layer      int           `json:"layer"`
	Bin        int           `json:"bin"`
	Categories []CategoryRef `json:"categories"`
	Pieces     int           `json:"pieces"` // delivered since its bins started afresh
}

// Plan is a collection's sorting plan: the groups it's being sorted into.
type Plan struct {
	Collection Collection     `json:"collection"`
	Profile    *StoredProfile `json:"profile"`
	Bins       []PlanBin      `json:"bins"`
	// Bucket is the pieces that went to the bucket under the tower (no bin).
	Bucket int `json:"bucket"`
}

// Plan is the collection's current plan: the bins its profile gave categories to,
// with how many pieces each has had.
func (c *Collections) Plan(ctx context.Context, id int64) (Plan, error) {
	col, err := c.store.GetCollection(ctx, id)
	if err != nil {
		return Plan{}, err
	}
	counts, err := c.store.CollectionBins(ctx, id)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Collection: col, Bins: []PlanBin{}, Bucket: counts[BinKey{}]}
	var bins Assignments
	if col.ProfileID != nil {
		stored, names, assigned, err := c.sorting.planOf(ctx, id, *col.ProfileID)
		if err != nil {
			return Plan{}, err
		}
		plan.Profile, bins = &stored, assigned
		for key, cats := range bins {
			pb := PlanBin{Layer: key.Layer, Bin: key.Bin, Categories: []CategoryRef{}, Pieces: counts[key]}
			for _, cat := range cats {
				pb.Categories = append(pb.Categories, CategoryRef{cat, names(cat)})
			}
			plan.Bins = append(plan.Bins, pb)
		}
	}
	// Bins that took pieces but have no categories now (moved by hand) still count.
	for key, n := range counts {
		if key == (BinKey{}) || bins[key] != nil {
			continue
		}
		plan.Bins = append(plan.Bins, PlanBin{Layer: key.Layer, Bin: key.Bin, Categories: []CategoryRef{}, Pieces: n})
	}
	slices.SortFunc(plan.Bins, func(a, b PlanBin) int {
		if a.Layer != b.Layer {
			return a.Layer - b.Layer
		}
		return a.Bin - b.Bin
	})
	return plan, nil
}

// SelectCollection makes a collection current, ready to sort into. Picking another
// collection than the current one means the bins were emptied: every bin counts from
// zero and the collection's bin layout starts afresh.
func (s *SortingService) SelectCollection(ctx context.Context, col Collection) error {
	if col.ArchivedAt != nil {
		return ErrCollectionArchived
	}
	s.mu.Lock()
	same := s.collection != nil && s.collection.ID == col.ID
	counts := s.counts
	s.mu.Unlock()
	if same {
		return nil
	}
	now := time.Now().UTC()
	if err := s.store.SelectCollection(ctx, col.ID, now); err != nil {
		return err
	}
	if counts != nil {
		if err := counts.EmptyAllBins(ctx, now); err != nil {
			return fmt.Errorf("empty the bins: %w", err)
		}
	}
	s.mu.Lock()
	s.collection = &CollectionRef{ID: col.ID, Name: col.Name, ProfileID: col.ProfileID}
	s.binCounts = BinCounts{}
	s.active = nil
	s.mu.Unlock()
	slog.Info("collection picked; bins start afresh", "collection", col.ID, "name", col.Name)
	if col.ProfileID != nil {
		return s.load(ctx, *col.ProfileID)
	}
	return nil
}

// Collection is the current collection, or nil.
func (s *SortingService) Collection() *CollectionRef {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.collection == nil {
		return nil
	}
	c := *s.collection
	return &c
}

func (s *SortingService) renamedCollection(id int64, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.collection != nil && s.collection.ID == id {
		s.collection.Name = name
	}
}

// planOf is a collection's bins under a profile, with the profile's category names.
// The active profile is already parsed; any other is read for its names.
func (s *SortingService) planOf(ctx context.Context, collectionID, profileID int64) (StoredProfile, func(string) string, Assignments, error) {
	bins, err := s.store.BinCategories(ctx, collectionID, profileID)
	if err != nil {
		return StoredProfile{}, nil, nil, err
	}
	s.mu.Lock()
	active, colorNames := s.active, s.colorNames
	s.mu.Unlock()
	if active != nil && active.stored.ID == profileID {
		return active.stored, namer(active.profile, colorNames), bins, nil
	}
	stored, p, err := s.parse(ctx, profileID)
	if err != nil {
		return StoredProfile{}, nil, nil, err
	}
	return stored, namer(p, colorNames), bins, nil
}
