package logic

import (
	"context"
	"encoding/json"
	"time"
)

// What people see. Every read goes through Access first.

// Access checks a session may see a collection: the admin sees all, a viewer the ones
// they were given. Anyone else gets ErrNotFound, so it can't tell what exists.
func (s *Service) Access(ctx context.Context, sess Session, collectionID int64) error {
	if sess.Admin() {
		return nil
	}
	if sess.ViewerID == 0 {
		return ErrNotFound
	}
	ok, err := s.store.CanSee(ctx, sess.ViewerID, collectionID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// Collections are the ones the session may see.
func (s *Service) Collections(ctx context.Context, sess Session) ([]Collection, error) {
	if sess.Admin() {
		return s.store.ListCollections(ctx, 0)
	}
	if sess.ViewerID == 0 {
		return []Collection{}, nil
	}
	return s.store.ListCollections(ctx, sess.ViewerID)
}

func (s *Service) Collection(ctx context.Context, sess Session, id int64) (Collection, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return Collection{}, err
	}
	return s.store.GetCollection(ctx, id)
}

// PlanView is a collection's sorting plan with each bin's pieces so far.
type PlanView struct {
	// Plan is what the sorter sent: {profile, bins: [{layer, bin, categories}], bins_started_at}.
	Plan json.RawMessage `json:"plan"`
	// Counts are each bin's pieces since its bins started afresh (layer 0: the bucket).
	Counts []BinCount `json:"counts"`
}

func (s *Service) Plan(ctx context.Context, sess Session, id int64) (PlanView, error) {
	col, err := s.Collection(ctx, sess, id)
	if err != nil {
		return PlanView{}, err
	}
	var plan struct {
		BinsStartedAt *time.Time `json:"bins_started_at"`
	}
	_ = json.Unmarshal(col.Plan, &plan)
	counts, err := s.store.BinCounts(ctx, id, plan.BinsStartedAt)
	if err != nil {
		return PlanView{}, err
	}
	return PlanView{Plan: col.Plan, Counts: counts}, nil
}

func (s *Service) Pieces(ctx context.Context, sess Session, id int64, f PieceFilter) ([]Piece, int, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return nil, 0, err
	}
	return s.store.ListPieces(ctx, id, f)
}

func (s *Service) Parts(ctx context.Context, sess Session, id int64) ([]PartCount, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return nil, err
	}
	return s.store.PartCounts(ctx, id)
}

func (s *Service) Runs(ctx context.Context, sess Session, id int64) ([]Run, error) {
	if err := s.Access(ctx, sess, id); err != nil {
		return nil, err
	}
	return s.store.ListRuns(ctx, id)
}

// Photo is a piece's photo file, if the session may see its collection.
func (s *Service) Photo(ctx context.Context, sess Session, pieceID string) (string, error) {
	piece, rel, err := s.store.GetPiece(ctx, pieceID)
	if err != nil {
		return "", err
	}
	if err := s.Access(ctx, sess, piece.CollectionID); err != nil {
		return "", err
	}
	path, ok := s.PhotoPath(rel)
	if !ok {
		return "", ErrNotFound
	}
	return path, nil
}
