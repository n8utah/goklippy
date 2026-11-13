package models

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func identified(t *testing.T, runID int64, part, color, colorName string, at time.Time) sorter.Piece {
	t.Helper()
	pred, err := json.Marshal(brickognize.Result{Items: []brickognize.Item{
		{ID: "other", ImgURL: "https://img/other.png"},
		{ID: part, ImgURL: "https://img/" + part + ".png"},
	}})
	require.NoError(t, err)
	return sorter.Piece{
		ID: sorter.NewPieceID(), RunID: runID, Status: sorter.PieceDistributed, SeenAt: at, DistributedAt: &at,
		PartID: part, PartName: "Brick " + part, PartCategory: "Brick", PartScore: 0.9,
		ColorID: color, ColorName: colorName, Prediction: pred, Problems: []sorter.Problem{},
	}
}

// Pieces are counted per part and colour and per run, once each; filling from the
// records adds only the ones not counted; clearing empties it.
func TestCatalogCountsFiltersFillsAndClears(t *testing.T) {
	ctx := context.Background()
	store := NewRecordsStore(openTestDB(t))
	t0 := time.Now().UTC().Truncate(time.Second)
	run1, err := store.StartRun(ctx, nil, "", t0)
	require.NoError(t, err)
	run2, err := store.StartRun(ctx, nil, "", t0.Add(time.Hour))
	require.NoError(t, err)

	a := identified(t, run1, "3001", "5", "Red", t0)
	b := identified(t, run2, "3001", "5", "Red", t0.Add(time.Hour))
	c := identified(t, run2, "3003", "11", "Black", t0.Add(time.Hour))
	for _, p := range []sorter.Piece{a, b, a} { // a twice: counted once
		sg, ok := sorter.SightingOf(p)
		require.True(t, ok)
		require.NoError(t, store.NoteCatalog(ctx, sg))
	}

	list, err := store.ListCatalog(ctx, sorter.CatalogFilter{Desc: true})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 2, list[0].Seen)
	assert.Equal(t, "https://img/3001.png", list[0].ImgURL, "the picture of the part named, not the first one")
	assert.Equal(t, []sorter.CatalogRun{{RunID: run2, Seen: 1}, {RunID: run1, Seen: 1}}, list[0].Runs)
	assert.True(t, list[0].FirstSeen.Equal(t0))
	assert.True(t, list[0].LastSeen.Equal(t0.Add(time.Hour)))

	// c is only in the records: filling adds it, and only it.
	for _, p := range []sorter.Piece{a, b, c} {
		require.NoError(t, store.SavePiece(ctx, p))
	}
	unidentified := sorter.Piece{ID: sorter.NewPieceID(), Status: sorter.PieceUnidentified, SeenAt: t0}
	require.NoError(t, store.SavePiece(ctx, unidentified))
	added, err := store.FillCatalog(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, added)

	byRun, err := store.ListCatalog(ctx, sorter.CatalogFilter{RunID: run1})
	require.NoError(t, err)
	require.Len(t, byRun, 1)
	assert.Equal(t, "3001", byRun[0].PartID)
	black, err := store.ListCatalog(ctx, sorter.CatalogFilter{ColorID: "11"})
	require.NoError(t, err)
	require.Len(t, black, 1)
	found, err := store.ListCatalog(ctx, sorter.CatalogFilter{Search: "3003"})
	require.NoError(t, err)
	require.Len(t, found, 1)
	byName, err := store.ListCatalog(ctx, sorter.CatalogFilter{Sort: "part"})
	require.NoError(t, err)
	require.Len(t, byName, 2)
	assert.Equal(t, "3001", byName[0].PartID)

	facets, err := store.CatalogFacets(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, facets.Parts)
	assert.Equal(t, 3, facets.Pieces)
	require.Len(t, facets.Colors, 2)
	assert.Equal(t, sorter.CatalogFacet{ID: "5", Name: "Red", Count: 2}, facets.Colors[0])
	require.Len(t, facets.Runs, 2)
	assert.Equal(t, 2, facets.Runs[0].Count, "run 2: b and c")
	assert.NotNil(t, facets.Runs[0].At)

	// A part's camera photo is from its latest piece with photos kept.
	require.NoError(t, store.AddPieceImage(ctx, a.ID, sorter.PieceImage{Index: 0, Path: "pieces/a/00.jpg", TakenAt: t0}))
	require.NoError(t, store.AddPieceImage(ctx, b.ID, sorter.PieceImage{Index: 2, Path: "pieces/b/02.jpg", Used: true, TakenAt: t0}))
	require.NoError(t, store.AddPieceImage(ctx, b.ID, sorter.PieceImage{Index: 1, Path: "pieces/b/01.jpg", TakenAt: t0}))
	withPhoto, err := store.ListCatalog(ctx, sorter.CatalogFilter{Search: "3001"})
	require.NoError(t, err)
	require.Len(t, withPhoto, 1)
	assert.Equal(t, b.ID, withPhoto[0].PhotoPieceID, "the latest piece")
	assert.Equal(t, 2, withPhoto[0].PhotoIndex, "one sent to Brickognize")
	pieces, _, err := store.ListPieces(ctx, sorter.PieceFilter{ColorID: "5"})
	require.NoError(t, err)
	require.Len(t, pieces, 2)
	assert.Equal(t, "https://img/3001.png", pieces[0].PartImgURL)

	full, err := store.CatalogPart(ctx, "3001", "5")
	require.NoError(t, err)
	assert.Equal(t, 2, full.Seen)
	require.Len(t, full.Photos, 3)
	assert.Equal(t, b.ID, full.Photos[0].PieceID)
	assert.Equal(t, run2, full.Photos[0].RunID)
	require.Len(t, full.Pieces, 2)
	assert.Equal(t, b.ID, full.Pieces[0].ID, "newest first")
	_, err = store.CatalogPart(ctx, "3001", "11")
	assert.ErrorIs(t, err, sorter.ErrPartNotFound)

	n, err := store.ClearPieceImages(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	withPhoto, err = store.ListCatalog(ctx, sorter.CatalogFilter{Search: "3001"})
	require.NoError(t, err)
	assert.Empty(t, withPhoto[0].PhotoPieceID, "no photos kept: Brickognize's picture only")

	require.NoError(t, store.ClearCatalog(ctx))
	list, err = store.ListCatalog(ctx, sorter.CatalogFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
	added, err = store.FillCatalog(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, added, "cleared, the records count again")
}
