package logic

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Which LEGO sets a collection could build. The sets and their parts are a separate
// database (the set catalog, sets.db), built from Rebrickable's downloads and swapped in
// whole; a collection's part counts are matched against it.

var (
	ErrSetsNotReady = errors.New("the set data isn't ready yet; it's downloaded when the site starts, so try again in a few minutes")
	ErrSetsBuilding = errors.New("the set data is already being rebuilt")
)

// setsMaxAge is how old the set data gets before it's rebuilt by itself.
const setsMaxAge = 30 * 24 * time.Hour

// HaveLine is how many of a part, in a colour, a collection has. Part is the BrickLink
// part number and Color the colour's name key (ColorKey), as the catalog matches them.
type HaveLine struct {
	Part  string `json:"p"`
	Color string `json:"c"`
	N     int    `json:"n"`
}

// SetFilter narrows and orders the ranking. AnyColour ranks by parts in any colour.
type SetFilter struct {
	Search    string
	MinParts  int
	AnyColour bool
	Limit     int
}

// SetMatch is a set and how much of it a collection has: Have parts in the right
// colour, HaveAnyColour counting a part in any colour, out of Need.
type SetMatch struct {
	SetNum        string `json:"set_num"`
	Name          string `json:"name"`
	Year          int    `json:"year"`
	Theme         string `json:"theme"`
	ImgURL        string `json:"img_url"`
	Need          int    `json:"need"`
	Have          int    `json:"have"`
	HaveAnyColour int    `json:"have_any_colour"`
}

// SetPartLine is one of a set's parts: how many it takes, how many the collection has in
// that colour (up to Need), and how many of the same part it has in other colours.
type SetPartLine struct {
	PartNum      string `json:"part_num"`
	PartName     string `json:"part_name"`
	ColorID      int64  `json:"color_id"`
	ColorName    string `json:"color_name"`
	RGB          string `json:"rgb"`
	ImgURL       string `json:"img_url"`
	Need         int    `json:"need"`
	Have         int    `json:"have"`
	OtherColours int    `json:"other_colours"`
}

// SetCoverage is how much of a collection the catalog knows: of its identified pieces,
// how many are parts it has (Known) and how many also in a colour it has
// (KnownColour). Unknown lists the most common part numbers it doesn't have.
type SetCoverage struct {
	Pieces      int           `json:"pieces" db:"pieces"`
	Known       int           `json:"known" db:"known"`
	KnownColour int           `json:"known_colour" db:"known_colour"`
	Unknown     []UnknownPart `json:"unknown"`
}

type UnknownPart struct {
	PartNum string `json:"part_num" db:"part_num"`
	Pieces  int    `json:"pieces" db:"pieces"`
}

// SetsStatus is the set data, for the admin and under the Sets tab.
type SetsStatus struct {
	Ready    bool       `json:"ready"`
	BuiltAt  *time.Time `json:"built_at,omitempty"`
	Sets     int        `json:"sets"`
	Building bool       `json:"building"`
	Error    string     `json:"error,omitempty"`
}

// SetCatalog is an open set catalog (models.Catalog).
type SetCatalog interface {
	ScoreSets(ctx context.Context, have []HaveLine) ([]SetMatch, error)
	SetParts(ctx context.Context, setNum string, have []HaveLine) (SetMatch, []SetPartLine, error)
	Coverage(ctx context.Context, have []HaveLine) (SetCoverage, error)
	Built(ctx context.Context) (time.Time, int, error)
	Close() error
}

// CatalogBuilder writes a new set catalog (models.CatalogBuilder). Finish completes
// it; Abort throws it away.
type CatalogBuilder interface {
	AddColor(id int64, name, nameKey, rgb string, trans bool) error
	AddTheme(id int64, name string, parentID int64) error
	AddPart(id int64, num, name string, categoryID, canonID int64) error
	AddSet(id int64, num, name string, year int, themeID int64, imgURL string) error
	AddSetPart(setID, partID, colorID int64, qty int) error
	AddImage(partID, colorID int64, imgURL string) error
	Finish(meta map[string]string) error
	Abort()
}

// SetsConfig is where the set catalog lives and how it's opened, built and sourced.
type SetsConfig struct {
	Path       string
	Open       func(path string) (SetCatalog, error)
	NewBuilder func(path string) (CatalogBuilder, error)
	Source     SetSource
}

// sets holds the open catalog; it's swapped while requests may be reading it.
type sets struct {
	cfg      SetsConfig
	mu       sync.RWMutex
	cat      SetCatalog
	building bool
	lastErr  string
	closed   bool // CloseSets was called

	cacheMu sync.Mutex
	scores  map[int64]scored // by collection
}

// UseSets turns on buildable sets. Without it, set requests say the data isn't ready.
func (s *Service) UseSets(cfg SetsConfig) {
	s.sets = &sets{cfg: cfg, scores: map[int64]scored{}}
	if _, err := os.Stat(cfg.Path); err != nil {
		return
	}
	cat, err := cfg.Open(cfg.Path)
	if err != nil {
		slog.Warn("opening the set data failed; it will be rebuilt", "path", cfg.Path, "error", err)
		return
	}
	s.sets.cat = cat
}

// CloseSets closes the set data, on shutdown.
func (s *Service) CloseSets() {
	if s.sets == nil {
		return
	}
	s.sets.mu.Lock()
	defer s.sets.mu.Unlock()
	s.sets.closed = true
	if s.sets.cat != nil {
		_ = s.sets.cat.Close()
		s.sets.cat = nil
	}
}

// KeepSetsFresh builds the set data if there's none or it's over a month old, checking
// daily until ctx ends.
func (s *Service) KeepSetsFresh(ctx context.Context) {
	if s.sets == nil {
		return
	}
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		st := s.SetsStatus(ctx)
		if !st.Building && (st.BuiltAt == nil || s.now().Sub(*st.BuiltAt) > setsMaxAge) {
			if err := s.BuildSets(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("building the set data failed", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RefreshSets rebuilds the set data in the background.
func (s *Service) RefreshSets() error {
	if s.sets == nil {
		return ErrSetsNotReady
	}
	s.sets.mu.RLock()
	building := s.sets.building
	s.sets.mu.RUnlock()
	if building {
		return ErrSetsBuilding
	}
	go func() {
		if err := s.BuildSets(context.Background()); err != nil && !errors.Is(err, ErrSetsBuilding) {
			slog.Warn("rebuilding the set data failed", "error", err)
		}
	}()
	return nil
}

func (s *Service) SetsStatus(ctx context.Context) SetsStatus {
	if s.sets == nil {
		return SetsStatus{}
	}
	s.sets.mu.RLock()
	defer s.sets.mu.RUnlock()
	st := SetsStatus{Building: s.sets.building, Error: s.sets.lastErr}
	if s.sets.cat != nil {
		if at, n, err := s.sets.cat.Built(ctx); err == nil {
			st.Ready, st.BuiltAt, st.Sets = true, &at, n
		}
	}
	return st
}

// BuildSets builds a new catalog next to the current one and swaps it in. Requests keep
// using the old one until the swap.
func (s *Service) BuildSets(ctx context.Context) error {
	ss := s.sets
	if ss == nil {
		return ErrSetsNotReady
	}
	ss.mu.Lock()
	if ss.building {
		ss.mu.Unlock()
		return ErrSetsBuilding
	}
	ss.building = true
	ss.mu.Unlock()

	started := s.now()
	slog.Info("building the set data", "path", ss.cfg.Path)
	err := s.buildSetsFile(ctx, ss.cfg.Path+".new")
	if err == nil {
		err = ss.swap()
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.building = false
	ss.lastErr = ""
	if err != nil {
		ss.lastErr = err.Error()
		return err
	}
	slog.Info("set data built", "took", s.now().Sub(started).Round(time.Second))
	return nil
}

func (s *Service) buildSetsFile(ctx context.Context, path string) error {
	b, err := s.sets.cfg.NewBuilder(path)
	if err != nil {
		return fmt.Errorf("start the set data: %w", err)
	}
	if err := importRebrickable(ctx, s.sets.cfg.Source, b); err != nil {
		b.Abort()
		return err
	}
	meta := map[string]string{"built_at": s.now().UTC().Format(time.RFC3339), "source": "rebrickable"}
	if err := b.Finish(meta); err != nil {
		return fmt.Errorf("finish the set data: %w", err)
	}
	return nil
}

// swap closes the current catalog, moves the new file over it and opens it. The old
// one has to be closed first: Windows won't replace an open file.
func (ss *sets) swap() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.cat != nil {
		_ = ss.cat.Close()
		ss.cat = nil
	}
	if err := os.Rename(ss.cfg.Path+".new", ss.cfg.Path); err != nil {
		return fmt.Errorf("swap in the new set data: %w", err)
	}
	if ss.closed {
		return nil // shutting down: it's opened on the next start
	}
	cat, err := ss.cfg.Open(ss.cfg.Path)
	if err != nil {
		return fmt.Errorf("open the new set data: %w", err)
	}
	ss.cat = cat
	ss.cacheMu.Lock()
	clear(ss.scores)
	ss.cacheMu.Unlock()
	return nil
}

// withCatalog runs fn with the open catalog, holding it open meanwhile.
func (s *Service) withCatalog(fn func(SetCatalog) error) error {
	if s.sets == nil {
		return ErrSetsNotReady
	}
	s.sets.mu.RLock()
	defer s.sets.mu.RUnlock()
	if s.sets.cat == nil {
		return ErrSetsNotReady
	}
	return fn(s.sets.cat)
}

// have is a collection's identified pieces, by part and colour, for the catalog.
func (s *Service) have(ctx context.Context, collectionID int64) ([]HaveLine, error) {
	counts, err := s.store.PartCounts(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	out := make([]HaveLine, 0, len(counts))
	for _, c := range counts {
		out = append(out, HaveLine{Part: c.PartID, Color: ColorKey(c.ColorName), N: c.Count})
	}
	return out, nil
}

// SetsPage is the best-covered sets that pass a filter, out of Total that do, and how
// much of the collection the set data knows.
type SetsPage struct {
	Sets     []SetMatch  `json:"sets"`
	Total    int         `json:"total"`
	Coverage SetCoverage `json:"coverage"`
}

// scored is every set a collection has a part of, worked out for one version of its
// counts (key) and kept until the counts or the set data change.
type scored struct {
	key      uint64
	sets     []SetMatch
	coverage SetCoverage
}

// BuildableSets are the sets a collection covers best. Scoring every set takes a
// moment on a big collection, so it's done once per change to the collection's counts;
// filtering and ordering are done here.
func (s *Service) BuildableSets(ctx context.Context, sess Session, id int64, f SetFilter) (SetsPage, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return SetsPage{}, err
	}
	have, err := s.have(ctx, id)
	if err != nil {
		return SetsPage{}, err
	}
	sc, err := s.scores(ctx, id, have)
	if err != nil {
		return SetsPage{}, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 60
	}
	list, total := rankSets(sc.sets, f)
	return SetsPage{Sets: list, Total: total, Coverage: sc.coverage}, nil
}

func (s *Service) scores(ctx context.Context, id int64, have []HaveLine) (scored, error) {
	h := fnv.New64a()
	for _, l := range have {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", l.Part, l.Color, l.N)
	}
	key := h.Sum64()
	var sc scored
	err := s.withCatalog(func(cat SetCatalog) error {
		s.sets.cacheMu.Lock()
		cached, ok := s.sets.scores[id]
		s.sets.cacheMu.Unlock()
		if ok && cached.key == key {
			sc = cached
			return nil
		}
		all, err := cat.ScoreSets(ctx, have)
		if err != nil {
			return err
		}
		cov, err := cat.Coverage(ctx, have)
		if err != nil {
			return err
		}
		sc = scored{key: key, sets: all, coverage: cov}
		s.sets.cacheMu.Lock()
		s.sets.scores[id] = sc
		s.sets.cacheMu.Unlock()
		return nil
	})
	return sc, err
}

// rankSets filters all and orders it best first (by share of parts in the right colour,
// or in any colour), returning the first f.Limit and how many passed.
func rankSets(all []SetMatch, f SetFilter) ([]SetMatch, int) {
	needle := strings.ToLower(strings.TrimSpace(f.Search))
	list := make([]SetMatch, 0, len(all))
	for _, m := range all {
		if m.Need < f.MinParts {
			continue
		}
		if needle != "" && !strings.HasPrefix(strings.ToLower(m.SetNum), needle) &&
			!strings.Contains(strings.ToLower(m.Name), needle) && !strings.Contains(strings.ToLower(m.Theme), needle) {
			continue
		}
		list = append(list, m)
	}
	first, second := func(m SetMatch) int { return m.Have }, func(m SetMatch) int { return m.HaveAnyColour }
	if f.AnyColour {
		first, second = second, first
	}
	// a/b against c/d, as a*d against c*b, so equal shares tie exactly.
	cmpShare := func(a, b SetMatch, by func(SetMatch) int) int {
		return cmp.Compare(by(b)*a.Need, by(a)*b.Need)
	}
	slices.SortFunc(list, func(a, b SetMatch) int {
		if c := cmpShare(a, b, first); c != 0 {
			return c
		}
		if c := cmpShare(a, b, second); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Need, a.Need); c != 0 {
			return c
		}
		return strings.Compare(a.SetNum, b.SetNum)
	})
	return list[:min(len(list), f.Limit)], len(list)
}

// SetDetail is one set's parts against a collection.
func (s *Service) SetDetail(ctx context.Context, sess Session, id int64, setNum string) (SetMatch, []SetPartLine, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return SetMatch{}, nil, err
	}
	have, err := s.have(ctx, id)
	if err != nil {
		return SetMatch{}, nil, err
	}
	var set SetMatch
	var lines []SetPartLine
	err = s.withCatalog(func(cat SetCatalog) error {
		var err error
		set, lines, err = cat.SetParts(ctx, setNum, have)
		return err
	})
	return set, lines, err
}

// ColorKey folds a colour name for matching across BrickLink and Rebrickable:
// "Light Bluish Gray" and "light-bluish gray" both give "lightbluishgray".
func ColorKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
