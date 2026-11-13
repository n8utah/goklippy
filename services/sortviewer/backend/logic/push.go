package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // a photo may come as PNG
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// What the sorter pushes. Every push is an upsert keyed by the sorter's own ids, so a
// retried or repeated push changes nothing.

// MaxPhotoBytes is the largest photo upload taken.
const MaxPhotoBytes = 10 << 20

// photoMaxSide is the longest side a stored photo is shrunk to.
const photoMaxSide = 800

func (s *Service) PushCollection(ctx context.Context, id int64, c PushCollection) error {
	c.Name = strings.TrimSpace(c.Name)
	if id <= 0 || c.Name == "" {
		return fmt.Errorf("%w: a collection needs an id and a name", ErrBadInput)
	}
	if len(c.Plan) > 0 && !json.Valid(c.Plan) {
		return fmt.Errorf("%w: plan isn't JSON", ErrBadInput)
	}
	if err := s.store.UpsertCollection(ctx, id, c.Name, c.Plan, s.now()); err != nil {
		return err
	}
	s.Events.Publish(Event{Type: "plan", CollectionID: id})
	return nil
}

func (s *Service) PushRun(ctx context.Context, id int64, r PushRun) error {
	if id <= 0 || r.CollectionID <= 0 || r.StartedAt.IsZero() {
		return fmt.Errorf("%w: a run needs an id, collection_id and started_at", ErrBadInput)
	}
	now := s.now()
	if err := s.store.EnsureCollection(ctx, r.CollectionID, now); err != nil {
		return err
	}
	if err := s.store.UpsertRun(ctx, id, r); err != nil {
		return err
	}
	_ = s.store.TouchCollection(ctx, r.CollectionID, now)
	s.Events.Publish(Event{Type: "run", CollectionID: r.CollectionID})
	return nil
}

func (s *Service) PushPiece(ctx context.Context, id string, p PushPiece) error {
	if !validPieceID(id) || p.CollectionID <= 0 || p.SeenAt.IsZero() {
		return fmt.Errorf("%w: a piece needs an id, collection_id and seen_at", ErrBadInput)
	}
	now := s.now()
	if err := s.store.EnsureCollection(ctx, p.CollectionID, now); err != nil {
		return err
	}
	if err := s.store.UpsertPiece(ctx, id, p, now); err != nil {
		return err
	}
	_ = s.store.TouchCollection(ctx, p.CollectionID, now)
	if piece, _, err := s.store.GetPiece(ctx, id); err == nil {
		s.Events.Publish(Event{Type: "piece", CollectionID: p.CollectionID, Piece: &piece})
	}
	return nil
}

// PushPhoto stores a piece's photo (JPEG or PNG), shrunk to photoMaxSide, replacing
// any it had. The piece must have been pushed first.
func (s *Service) PushPhoto(ctx context.Context, id string, data []byte) error {
	if !validPieceID(id) {
		return fmt.Errorf("%w: bad piece id", ErrBadInput)
	}
	piece, _, err := s.store.GetPiece(ctx, id)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: not a JPEG or PNG photo", ErrBadInput)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, shrink(img, photoMaxSide), &jpeg.Options{Quality: 82}); err != nil {
		return err
	}
	rel := filepath.ToSlash(filepath.Join("photos", id[:2], id+".jpg"))
	path, _ := s.PhotoPath(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return err
	}
	old, err := s.store.SetPiecePhoto(ctx, id, rel)
	if err != nil {
		return err
	}
	if old != "" && old != rel {
		s.removePhotos([]string{old})
	}
	piece.HasPhoto = true
	s.Events.Publish(Event{Type: "piece", CollectionID: piece.CollectionID, Piece: &piece})
	return nil
}

// ClearUnknowns drops a collection's unknown pieces and their photos: the sorter is
// feeding them through again.
func (s *Service) ClearUnknowns(ctx context.Context, collectionID int64) (int, error) {
	if _, err := s.store.GetCollection(ctx, collectionID); err != nil {
		return 0, err
	}
	n, photos, err := s.store.ClearUnknowns(ctx, collectionID)
	if err != nil {
		return 0, err
	}
	s.removePhotos(photos)
	s.Events.Publish(Event{Type: "unknowns", CollectionID: collectionID})
	return n, nil
}

// shrink scales img down so its longest side is at most maxSide.
func shrink(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	if w >= h {
		h, w = h*maxSide/w, maxSide
	} else {
		w, h = w*maxSide/h, maxSide
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(1, w), max(1, h)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// validPieceID accepts the sorter's piece ids (UUIDs): letters, digits and dashes.
func validPieceID(id string) bool {
	if len(id) < 8 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
