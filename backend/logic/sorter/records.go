package sorter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// PieceStatus is how far a piece got and what became of it.
type PieceStatus string

const (
	PieceTracking     PieceStatus = "tracking"     // seen, not photographed yet
	PieceIdentifying  PieceStatus = "identifying"  // photos sent to Brickognize
	PieceIdentified   PieceStatus = "identified"   // has a part and color
	PieceUnidentified PieceStatus = "unidentified" // Brickognize found no usable match
	PieceIDFailed     PieceStatus = "id_failed"    // the request failed or timed out
	PieceMultiDrop    PieceStatus = "multi_drop"   // landed with another piece
	PieceDistributed  PieceStatus = "distributed"  // dropped into a bin or the bucket
	PieceLost         PieceStatus = "lost"         // the camera lost it
)

// Problem kinds recorded on pieces.
const (
	ProblemDoubleFeed        = "double_feed"
	ProblemIDFailed          = "id_failed"
	ProblemIDTimeout         = "id_timeout"
	ProblemNoBin             = "no_bin"
	ProblemNeverPhotographed = "never_photographed"
	ProblemDidntFall         = "didnt_fall"
	ProblemLost              = "lost"
	ProblemSweptOff          = "swept_off"
)

var ErrPieceNotFound = errors.New("piece not found")

type Problem struct {
	Kind    string    `json:"kind"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// Piece is everything recorded about one piece.
type Piece struct {
	ID            string      `json:"id"`
	RunID         int64       `json:"run_id,omitempty"`
	Status        PieceStatus `json:"status"`
	SeenAt        time.Time   `json:"seen_at"`
	IdentifiedAt  *time.Time  `json:"identified_at,omitempty"`
	DistributedAt *time.Time  `json:"distributed_at,omitempty"`

	PartID       string  `json:"part_id,omitempty"`
	PartName     string  `json:"part_name,omitempty"`
	PartCategory string  `json:"part_category,omitempty"`
	PartScore    float64 `json:"part_score,omitempty"`
	// PartImgURL is Brickognize's picture of the part (read from Prediction; not saved).
	PartImgURL string  `json:"part_img_url,omitempty"`
	ColorID    string  `json:"color_id,omitempty"`
	ColorName  string  `json:"color_name,omitempty"`
	ColorScore float64 `json:"color_score,omitempty"`

	Category     string `json:"category,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	// BinLayer and BinNumber are 0 for the bucket under the tower.
	BinLayer    int    `json:"bin_layer,omitempty"`
	BinNumber   int    `json:"bin_number,omitempty"`
	RouteReason string `json:"route_reason,omitempty"`
	// VirtualBin: the chute was left out, so the bin is where it would have gone.
	VirtualBin bool    `json:"virtual_bin,omitempty"`
	PieceMm    float64 `json:"piece_mm,omitempty"`

	Problems   []Problem       `json:"problems"`
	Prediction json.RawMessage `json:"prediction,omitempty"`
	Images     []PieceImage    `json:"images,omitempty"`
	Review     *PieceReview    `json:"review,omitempty"`
}

// Issue flags a reviewer can set on a piece's photos.
var ReviewIssues = []string{"not_lego", "multiple_pieces", "no_piece", "assembly", "entangled", "blurry"}

var ErrBadReview = errors.New("invalid review")

// PieceReview is a person's check of a piece's identification.
type PieceReview struct {
	PartCorrect  *bool     `json:"part_correct"`
	ColorCorrect *bool     `json:"color_correct"`
	ColorID      string    `json:"color_id,omitempty"` // the right color, when it was wrong
	Issues       []string  `json:"issues"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

// Check reports a review with unknown issue flags.
func (r PieceReview) Check() error {
	for _, issue := range r.Issues {
		if !slices.Contains(ReviewIssues, issue) {
			return fmt.Errorf("%w: unknown issue %q", ErrBadReview, issue)
		}
	}
	return nil
}

// AddProblem notes something that went wrong with the piece.
func (p *Piece) AddProblem(kind, message string, at time.Time) {
	p.Problems = append(p.Problems, Problem{Kind: kind, Message: message, At: at})
}

// Clone copies a piece so the copy can be saved while the original changes.
func (p Piece) Clone() Piece {
	out := p
	out.Problems = append([]Problem(nil), p.Problems...)
	out.Images = append([]PieceImage(nil), p.Images...)
	return out
}

type PieceImage struct {
	Index   int       `json:"index"`
	Path    string    `json:"path"` // relative to the data folder
	Used    bool      `json:"used"` // sent to Brickognize
	TakenAt time.Time `json:"taken_at"`
}

type Run struct {
	ID             int64      `json:"id"`
	CollectionID   int64      `json:"collection_id"`
	CollectionName string     `json:"collection_name"`
	ProfileID      *int64     `json:"profile_id,omitempty"`
	ProfileName    string     `json:"profile_name"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	Pieces         int        `json:"pieces"`
	Distributed    int        `json:"distributed"`
	// LastPieceAt is when its last piece was sorted: where its sorting really ended,
	// whatever ran on after (an empty hopper turning until it was stopped).
	LastPieceAt *time.Time `json:"last_piece_at,omitempty"`
}

type Incident struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	PieceID    string     `json:"piece_id,omitempty"`
	OpenedAt   time.Time  `json:"opened_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Resolution string     `json:"resolution,omitempty"`
	// Cameras are every channel's picture and state when it opened (incident_cameras.go).
	Cameras []IncidentCamera `json:"cameras,omitempty"`
	// Focus is where the problem is, when known; kept with the cameras' states.
	Focus []IncidentFocus `json:"focus,omitempty"`
}

// PieceFilter narrows the pieces list.
type PieceFilter struct {
	CollectionID int64
	RunID        int64
	Status       string
	Unknown      bool // only pieces with no part (see Unknown)
	ColorID      string
	PartCategory string // Brickognize's category of the part ("Brick", "Plate", …)
	Search       string // a part id, category name or piece id prefix
	Limit        int
	Offset       int
}

// Stats are lifetime totals.
type Stats struct {
	Pieces      int            `json:"pieces"`
	Distributed int            `json:"distributed"`
	ByStatus    map[string]int `json:"by_status"`
	Runs        int            `json:"runs"`
}

// RecordsStore keeps pieces, runs and incidents.
type RecordsStore interface {
	StartRun(ctx context.Context, collectionID int64, profileID *int64, profileName string, at time.Time) (int64, error)
	EndRun(ctx context.Context, id int64, at time.Time) error
	// ListRuns is the newest runs; collectionID 0 means every collection's.
	ListRuns(ctx context.Context, collectionID int64, limit int) ([]Run, error)
	SavePiece(ctx context.Context, p Piece) error
	AddPieceImage(ctx context.Context, pieceID string, img PieceImage) error
	ListPieces(ctx context.Context, f PieceFilter) ([]Piece, int, error)
	GetPiece(ctx context.Context, id string) (Piece, error)
	OpenIncident(ctx context.Context, inc Incident) (int64, error)
	ResolveIncident(ctx context.Context, id int64, resolution string, at time.Time) error
	ListIncidents(ctx context.Context, limit int) ([]Incident, error)
	Stats(ctx context.Context) (Stats, error)
	SaveReview(ctx context.Context, pieceID string, r PieceReview) error
}

// Records saves what the machine does without holding up the control loop: writes go
// on a queue that one goroutine drains in order. Reads go straight to the store.
type Records struct {
	store   RecordsStore
	dataDir string
	queue   chan func(context.Context)
	wg      sync.WaitGroup
}

const recordsQueue = 1024

// NewRecords stores photos under dataDir. Call Run to start writing.
func NewRecords(store RecordsStore, dataDir string) *Records {
	return &Records{store: store, dataDir: dataDir, queue: make(chan func(context.Context), recordsQueue)}
}

// Store is for reads.
func (r *Records) Store() RecordsStore { return r.store }

// DataDir is where photos are kept.
func (r *Records) DataDir() string { return r.dataDir }

// Run writes queued records until ctx ends, then writes what's left.
func (r *Records) Run(ctx context.Context) {
	for {
		select {
		case job := <-r.queue:
			job(context.Background())
			r.wg.Done()
		case <-ctx.Done():
			for {
				select {
				case job := <-r.queue:
					job(context.Background())
					r.wg.Done()
				default:
					return
				}
			}
		}
	}
}

// Flush waits for every queued write.
func (r *Records) Flush() {
	r.wg.Wait()
}

func (r *Records) enqueue(what string, job func(context.Context) error) {
	r.wg.Add(1)
	wrapped := func(ctx context.Context) {
		if err := job(ctx); err != nil {
			slog.Warn("sorter record not saved", "what", what, "error", err)
		}
	}
	select {
	case r.queue <- wrapped:
	default:
		r.wg.Done()
		slog.Warn("sorter record queue full; record dropped", "what", what)
	}
}

// SavePiece queues a copy of the piece to be saved (inserted or updated).
func (r *Records) SavePiece(p Piece) {
	cp := p.Clone()
	r.enqueue("piece "+p.ID, func(ctx context.Context) error { return r.store.SavePiece(ctx, cp) })
}

// SaveImage queues a photo of a piece: the JPEG goes to a file, its path to the store.
// The piece must have been saved (queued) first.
func (r *Records) SaveImage(pieceID string, index int, jpeg []byte, used bool, at time.Time) {
	rel := filepath.ToSlash(filepath.Join("pieces", pieceID[:2], pieceID, fmt.Sprintf("%02d.jpg", index)))
	r.enqueue("image "+rel, func(ctx context.Context) error {
		full := filepath.Join(r.dataDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, jpeg, 0o644); err != nil {
			return err
		}
		return r.store.AddPieceImage(ctx, pieceID, PieceImage{Index: index, Path: rel, Used: used, TakenAt: at})
	})
}

// ImagePath is a stored photo's file, refusing paths outside the data folder.
func (r *Records) ImagePath(rel string) (string, error) {
	full := filepath.Join(r.dataDir, filepath.FromSlash(rel))
	inside, err := filepath.Rel(r.dataDir, full)
	if err != nil || strings.HasPrefix(inside, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("bad image path")
	}
	return full, nil
}

// NewPieceID is a random UUID (version 4).
func NewPieceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
