package main

import (
	"context"
	"time"

	"goklippy/backend/logic/sorter"
	"goklippy/backend/models"
)

// The sort viewer only hears about what's saved, so the stores are wrapped: after each
// save that matters, the change is marked for sharing (a no-op unless its collection is
// shared). Everything else passes straight through.

// sharingRecords marks runs, pieces, photos, reviews, renames and cleared unknowns.
type sharingRecords struct {
	*models.RecordsStore
	share *sorter.Sharing
}

func (s sharingRecords) StartRun(ctx context.Context, collectionID int64, profileID *int64, profileName string, at time.Time) (int64, error) {
	id, err := s.RecordsStore.StartRun(ctx, collectionID, profileID, profileName, at)
	if err == nil {
		s.share.RunChanged(ctx, id)
	}
	return id, err
}

func (s sharingRecords) EndRun(ctx context.Context, id int64, at time.Time) error {
	err := s.RecordsStore.EndRun(ctx, id, at)
	if err == nil {
		s.share.RunChanged(ctx, id)
	}
	return err
}

func (s sharingRecords) SavePiece(ctx context.Context, p sorter.Piece) error {
	err := s.RecordsStore.SavePiece(ctx, p)
	if err == nil {
		s.share.PieceChanged(ctx, p.ID)
	}
	return err
}

func (s sharingRecords) AddPieceImage(ctx context.Context, pieceID string, img sorter.PieceImage) error {
	err := s.RecordsStore.AddPieceImage(ctx, pieceID, img)
	if err == nil {
		s.share.PhotoChanged(ctx, pieceID)
	}
	return err
}

func (s sharingRecords) SaveReview(ctx context.Context, pieceID string, r sorter.PieceReview) error {
	err := s.RecordsStore.SaveReview(ctx, pieceID, r)
	if err == nil {
		s.share.PieceChanged(ctx, pieceID)
	}
	return err
}

func (s sharingRecords) RenameCollection(ctx context.Context, id int64, name string) error {
	err := s.RecordsStore.RenameCollection(ctx, id, name)
	if err == nil {
		s.share.CollectionChanged(ctx, id)
	}
	return err
}

func (s sharingRecords) ClearUnknowns(ctx context.Context, id int64) ([]string, error) {
	ids, err := s.RecordsStore.ClearUnknowns(ctx, id)
	if err == nil {
		s.share.UnknownsCleared(ctx, id)
	}
	return ids, err
}

// sharingPlans marks a collection whenever its plan (its bins, or its profile) changes.
type sharingPlans struct {
	*models.SortingStore
	share *sorter.Sharing
}

func (s sharingPlans) SelectCollection(ctx context.Context, id int64, at time.Time) error {
	err := s.SortingStore.SelectCollection(ctx, id, at)
	if err == nil {
		s.share.CollectionChanged(ctx, id)
	}
	return err
}

func (s sharingPlans) SetCollectionProfile(ctx context.Context, collectionID, profileID int64) error {
	err := s.SortingStore.SetCollectionProfile(ctx, collectionID, profileID)
	if err == nil {
		s.share.CollectionChanged(ctx, collectionID)
	}
	return err
}

func (s sharingPlans) ReplaceBinCategories(ctx context.Context, collectionID, profileID int64, bins sorter.Assignments) error {
	err := s.SortingStore.ReplaceBinCategories(ctx, collectionID, profileID, bins)
	if err == nil {
		s.share.CollectionChanged(ctx, collectionID)
	}
	return err
}

func (s sharingPlans) SetBinCategories(ctx context.Context, collectionID, profileID int64, bin sorter.BinKey, categories []string) error {
	err := s.SortingStore.SetBinCategories(ctx, collectionID, profileID, bin, categories)
	if err == nil {
		s.share.CollectionChanged(ctx, collectionID)
	}
	return err
}
