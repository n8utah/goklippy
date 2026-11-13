package sorter

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"goklippy/backend/logic/brickognize"
)

// The catalog: every part and colour the machine has identified, across all runs, with
// how many, when, in which runs, and Brickognize's picture of the part. It's kept apart
// from the piece records (whose photos and rows may be pruned) and cleared on its own.
// Each identified piece is counted once, when it leaves the classification channel.

// CatalogEntry is one part in one colour.
type CatalogEntry struct {
	PartID    string       `json:"part_id"`
	ColorID   string       `json:"color_id"`
	PartName  string       `json:"part_name"`
	ColorName string       `json:"color_name"`
	Category  string       `json:"category"`
	ImgURL    string       `json:"img_url"`
	Seen      int          `json:"seen"`
	BestScore float64      `json:"best_score"`
	FirstSeen time.Time    `json:"first_seen"`
	LastSeen  time.Time    `json:"last_seen"`
	Runs      []CatalogRun `json:"runs"`
	// A camera photo of the part, from its latest piece whose photos are still kept
	// (empty when none is): GET /sorter/pieces/{PhotoPieceID}/images/{PhotoIndex}.
	PhotoPieceID string `json:"photo_piece_id,omitempty"`
	PhotoIndex   int    `json:"photo_index"`
}

// CatalogRun is how many of a part were seen in one run.
type CatalogRun struct {
	RunID int64 `json:"run_id"`
	Seen  int   `json:"seen"`
}

// CatalogFilter narrows and orders the catalog. Sort is "seen" (default), "name",
// "part", "color" or "last"; Desc reverses it.
type CatalogFilter struct {
	PartID   string // exactly this part
	Search   string
	ColorID  string
	Category string
	RunID    int64
	Sort     string
	Desc     bool
}

// CatalogFacets are the values the catalog can be filtered by, with counts.
type CatalogFacets struct {
	Colors     []CatalogFacet `json:"colors"`
	Categories []CatalogFacet `json:"categories"`
	Runs       []CatalogFacet `json:"runs"`
	Parts      int            `json:"parts"`  // distinct parts and colours
	Pieces     int            `json:"pieces"` // pieces counted
}

// CatalogFacet is one filter value: an id (a colour or run id; empty for categories),
// its name and how many pieces have it. At is when a run started.
type CatalogFacet struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Count int        `json:"count"`
	At    *time.Time `json:"at,omitempty"`
}

// CatalogSighting is one identified piece to count.
type CatalogSighting struct {
	PieceID   string
	RunID     int64
	PartID    string
	ColorID   string
	PartName  string
	ColorName string
	Category  string
	ImgURL    string
	Score     float64
	At        time.Time
}

// CatalogPhoto is one kept camera photo of a part.
type CatalogPhoto struct {
	PieceID string    `json:"piece_id"`
	Index   int       `json:"index"`
	Used    bool      `json:"used"` // sent to Brickognize
	RunID   int64     `json:"run_id,omitempty"`
	TakenAt time.Time `json:"taken_at"`
}

// CatalogPart is one part and colour in full: the catalog's entry, every camera photo
// still kept of it, and its pieces in the records (newest first).
type CatalogPart struct {
	CatalogEntry
	Photos []CatalogPhoto `json:"photos"`
	Pieces []Piece        `json:"pieces"`
}

var ErrPartNotFound = errors.New("that part isn't in the catalog")

// CatalogStore keeps the catalog.
type CatalogStore interface {
	// CatalogPart is ErrPartNotFound when the part and colour aren't in the catalog.
	CatalogPart(ctx context.Context, partID, colorID string) (CatalogPart, error)
	NoteCatalog(ctx context.Context, s CatalogSighting) error
	ListCatalog(ctx context.Context, f CatalogFilter) ([]CatalogEntry, error)
	CatalogFacets(ctx context.Context) (CatalogFacets, error)
	ClearCatalog(ctx context.Context) error
	// FillCatalog counts every identified piece in the records not yet counted, and
	// returns how many were added.
	FillCatalog(ctx context.Context) (int, error)
}

var ErrNoCatalog = errors.New("the catalog isn't available")

// SightingOf is a piece as the catalog counts it; false when it wasn't identified.
func SightingOf(p Piece) (CatalogSighting, bool) {
	if p.PartID == "" {
		return CatalogSighting{}, false
	}
	at := p.SeenAt
	if p.DistributedAt != nil {
		at = *p.DistributedAt
	} else if p.IdentifiedAt != nil {
		at = *p.IdentifiedAt
	}
	return CatalogSighting{
		PieceID: p.ID, RunID: p.RunID, PartID: p.PartID, ColorID: p.ColorID,
		PartName: p.PartName, ColorName: p.ColorName, Category: p.PartCategory,
		ImgURL: PartImage(p.Prediction, p.PartID), Score: p.PartScore, At: at,
	}, true
}

// PartImage is Brickognize's picture of the part, from the piece's saved answer.
func PartImage(prediction json.RawMessage, partID string) string {
	if partID == "" {
		return ""
	}
	if len(prediction) == 0 {
		return ""
	}
	var res brickognize.Result
	if json.Unmarshal(prediction, &res) != nil {
		return ""
	}
	for _, it := range res.Items {
		if it.ID == partID && it.ImgURL != "" {
			return it.ImgURL
		}
	}
	return ""
}

// Catalog is the store's catalog, when it keeps one.
func (r *Records) Catalog() (CatalogStore, error) {
	if c, ok := r.store.(CatalogStore); ok {
		return c, nil
	}
	return nil, ErrNoCatalog
}

// NoteCatalog counts an identified piece in the catalog, off the control loop.
func (r *Records) NoteCatalog(p Piece) {
	s, ok := SightingOf(p)
	if !ok {
		return
	}
	c, err := r.Catalog()
	if err != nil {
		return
	}
	r.enqueue("catalog "+p.ID, func(ctx context.Context) error { return c.NoteCatalog(ctx, s) })
}
