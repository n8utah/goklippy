package sorter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Piece photos can be cleared on their own: the records and the catalog stay, and the
// catalog falls back to Brickognize's pictures.

// PhotoStore forgets the photos recorded for pieces.
type PhotoStore interface {
	ClearPieceImages(ctx context.Context) (int, error)
}

var ErrNoPhotoStore = errors.New("piece photos can't be cleared here")

// PhotoUsage is what the piece photos take up on disk.
type PhotoUsage struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

func (r *Records) photosDir() string {
	return filepath.Join(r.dataDir, "pieces")
}

// Photos is how many piece photos are kept, and their size.
func (r *Records) Photos() PhotoUsage {
	var u PhotoUsage
	if r.dataDir == "" {
		return u
	}
	_ = filepath.WalkDir(r.photosDir(), func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			u.Files++
			u.Bytes += info.Size()
		}
		return nil
	})
	return u
}

// ClearPhotos removes every piece photo, after the photos already queued are written,
// and returns what was freed. Photos taken from now on are kept as usual.
func (r *Records) ClearPhotos(ctx context.Context) (PhotoUsage, error) {
	ps, ok := r.store.(PhotoStore)
	if !ok || r.dataDir == "" {
		return PhotoUsage{}, ErrNoPhotoStore
	}
	r.Flush()
	before := r.Photos()
	if _, err := ps.ClearPieceImages(ctx); err != nil {
		return PhotoUsage{}, err
	}
	if err := os.RemoveAll(r.photosDir()); err != nil {
		return PhotoUsage{}, err
	}
	return before, nil
}
