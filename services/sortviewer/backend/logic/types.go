// Package logic is the sort viewer's business: what the sorter pushes, who may see
// what, and signing in.
package logic

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrBadInput      = errors.New("invalid input")
	ErrEmailTaken    = errors.New("a viewer with that email already exists")
	ErrBadLogin      = errors.New("wrong email or password")
	ErrTooManyLogins = errors.New("too many sign-in attempts; wait a minute and try again")
)

// Collection is one person's LEGO, shared from the sorter, with its totals.
type Collection struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Plan is the sorter's current sorting plan: {profile, bins, bucket}.
	Plan       json.RawMessage `json:"plan"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Runs       int             `json:"runs"`
	Pieces     int             `json:"pieces"`
	Identified int             `json:"identified"`
	Unknown    int             `json:"unknown"`
	// Sorting: a run into it is on now.
	Sorting     bool       `json:"sorting"`
	LastPieceAt *time.Time `json:"last_piece_at,omitempty"`
}

type Run struct {
	ID           int64      `json:"id"`
	CollectionID int64      `json:"collection_id"`
	ProfileName  string     `json:"profile_name"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Pieces       int        `json:"pieces"`
}

// Piece is one piece the sorter saw. PartID is "" for an unknown.
type Piece struct {
	ID           string     `json:"id"`
	CollectionID int64      `json:"collection_id"`
	RunID        int64      `json:"run_id,omitempty"`
	Status       string     `json:"status"`
	SeenAt       time.Time  `json:"seen_at"`
	SortedAt     *time.Time `json:"sorted_at,omitempty"`
	PartID       string     `json:"part_id"`
	PartName     string     `json:"part_name"`
	PartCategory string     `json:"part_category"`
	PartScore    float64    `json:"part_score"`
	PartImgURL   string     `json:"part_img_url"`
	ColorID      string     `json:"color_id"`
	ColorName    string     `json:"color_name"`
	Category     string     `json:"category"`
	CategoryName string     `json:"category_name"`
	BinLayer     int        `json:"bin_layer"`
	BinNumber    int        `json:"bin_number"`
	VirtualBin   bool       `json:"virtual_bin"`
	HasPhoto     bool       `json:"has_photo"`
}

// PartCount is how many of one part and colour a collection has.
type PartCount struct {
	PartID       string `json:"part_id" db:"part_id"`
	PartName     string `json:"part_name" db:"part_name"`
	PartCategory string `json:"part_category" db:"part_category"`
	ColorID      string `json:"color_id" db:"color_id"`
	ColorName    string `json:"color_name" db:"color_name"`
	ImgURL       string `json:"img_url" db:"img_url"`
	Count        int    `json:"count" db:"count"`
	// PhotoPieceID is a piece of it with a photo, for its picture.
	PhotoPieceID string `json:"photo_piece_id,omitempty" db:"photo_piece_id"`
}

// BinCount is how many pieces a bin has had (layer 0 is the bucket).
type BinCount struct {
	Layer  int `json:"layer" db:"layer"`
	Bin    int `json:"bin" db:"bin"`
	Pieces int `json:"pieces" db:"pieces"`
}

// PieceFilter narrows a collection's pieces.
type PieceFilter struct {
	Unknown bool   // only pieces with no part
	Search  string // part number, name or colour
	Limit   int
	Offset  int
}

type Viewer struct {
	ID          int64     `json:"id" db:"id"`
	Email       string    `json:"email" db:"email"`
	Name        string    `json:"name" db:"name"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	Collections []int64   `json:"collections" db:"-"`
}

type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"` // the key's first characters, to tell keys apart
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Session is a signed-in browser: a viewer, or the admin (signed in through Gangway).
type Session struct {
	ViewerID   int64  `json:"viewer_id,omitempty"`
	ViewerName string `json:"-"`
	AdminEmail string `json:"admin_email,omitempty"`
	AdminName  string `json:"admin_name,omitempty"`
	OIDCSid    string `json:"-"`
	ExpiresAt  time.Time
}

func (s Session) Admin() bool { return s.AdminEmail != "" }

// What the sorter pushes. Ids are the sorter's own.

type PushCollection struct {
	Name string          `json:"name"`
	Plan json.RawMessage `json:"plan"`
}

type PushRun struct {
	CollectionID int64      `json:"collection_id"`
	ProfileName  string     `json:"profile_name"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
}

type PushPiece struct {
	CollectionID int64      `json:"collection_id"`
	RunID        int64      `json:"run_id"`
	Status       string     `json:"status"`
	SeenAt       time.Time  `json:"seen_at"`
	SortedAt     *time.Time `json:"sorted_at"`
	PartID       string     `json:"part_id"`
	PartName     string     `json:"part_name"`
	PartCategory string     `json:"part_category"`
	PartScore    float64    `json:"part_score"`
	PartImgURL   string     `json:"part_img_url"`
	ColorID      string     `json:"color_id"`
	ColorName    string     `json:"color_name"`
	Category     string     `json:"category"`
	CategoryName string     `json:"category_name"`
	BinLayer     int        `json:"bin_layer"`
	BinNumber    int        `json:"bin_number"`
	VirtualBin   bool       `json:"virtual_bin"`
}
