package models

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordsPiecesRunsAndIncidents(t *testing.T) {
	store := NewRecordsStore(openTestDB(t))
	records := sorter.NewRecords(store, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go records.Run(ctx)

	start := time.Now().UTC().Truncate(time.Second)
	runID, err := store.StartRun(ctx, newCollection(t, store, "Test"), nil, "Bricks", start)
	require.NoError(t, err)

	p := sorter.Piece{ID: sorter.NewPieceID(), RunID: runID, Status: sorter.PieceTracking, SeenAt: start}
	records.SavePiece(p)
	records.SaveImage(p.ID, 0, []byte("jpeg-0"), false, start)
	records.SaveImage(p.ID, 1, []byte("jpeg-1"), true, start)

	// The piece moves on: identified, then distributed, with a problem noted.
	identified := start.Add(2 * time.Second)
	p.Status, p.IdentifiedAt = sorter.PieceDistributed, &identified
	p.PartID, p.PartName, p.ColorID, p.ColorName, p.PartScore = "3001", "Brick 2 x 4", "5", "Red", 0.93
	p.Category, p.CategoryName, p.BinLayer, p.BinNumber, p.RouteReason = "bricks", "Bricks", 1, 4, sorter.RouteNewBin
	p.DistributedAt = &identified
	p.Prediction = json.RawMessage(`{"items": []}`)
	p.AddProblem(sorter.ProblemDidntFall, "pushed off on the second try", identified)
	records.SavePiece(p)

	other := sorter.Piece{ID: sorter.NewPieceID(), RunID: runID, Status: sorter.PieceMultiDrop, SeenAt: start.Add(time.Second)}
	records.SavePiece(other)
	records.Flush()

	got, err := store.GetPiece(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, sorter.PieceDistributed, got.Status)
	assert.Equal(t, "Brick 2 x 4", got.PartName)
	assert.Equal(t, 4, got.BinNumber)
	require.Len(t, got.Problems, 1)
	assert.Equal(t, sorter.ProblemDidntFall, got.Problems[0].Kind)
	assert.JSONEq(t, `{"items": []}`, string(got.Prediction))
	require.Len(t, got.Images, 2)
	assert.True(t, got.Images[1].Used)
	file, err := records.ImagePath(got.Images[1].Path)
	require.NoError(t, err)
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "jpeg-1", string(data))

	_, err = records.ImagePath("../../etc/passwd")
	assert.Error(t, err)
	_, err = store.GetPiece(ctx, "missing")
	assert.ErrorIs(t, err, sorter.ErrPieceNotFound)

	list, total, err := store.ListPieces(ctx, sorter.PieceFilter{RunID: runID})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, other.ID, list[0].ID, "newest first")
	require.Len(t, list[1].Images, 1, "the list carries one photo")
	assert.True(t, list[1].Images[0].Used, "the one Brickognize used")

	list, total, err = store.ListPieces(ctx, sorter.PieceFilter{Search: "3001"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, p.ID, list[0].ID)
	_, total, err = store.ListPieces(ctx, sorter.PieceFilter{Status: "multi_drop"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	require.NoError(t, store.EndRun(ctx, runID, start.Add(time.Minute)))
	runs, err := store.ListRuns(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 2, runs[0].Pieces)
	assert.Equal(t, 1, runs[0].Distributed)
	require.NotNil(t, runs[0].EndedAt)

	incID, err := store.OpenIncident(ctx, sorter.Incident{Kind: "no_bin", Title: "No bin", PieceID: p.ID, OpenedAt: start})
	require.NoError(t, err)
	require.NoError(t, store.ResolveIncident(ctx, incID, "send_to_bucket", start.Add(time.Minute)))
	incs, err := store.ListIncidents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, incs, 1)
	assert.Equal(t, "send_to_bucket", incs[0].Resolution)
	assert.NotNil(t, incs[0].ResolvedAt)

	stats, err := store.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Pieces)
	assert.Equal(t, 1, stats.Distributed)
	assert.Equal(t, 1, stats.Runs)

	assert.FileExists(t, filepath.Join(records.DataDir(), "pieces", p.ID[:2], p.ID, "00.jpg"))
}

func TestPieceReviews(t *testing.T) {
	store := NewRecordsStore(openTestDB(t))
	ctx := context.Background()
	p := sorter.Piece{ID: sorter.NewPieceID(), Status: sorter.PieceDistributed, SeenAt: time.Now()}
	require.NoError(t, store.SavePiece(ctx, p))

	got, err := store.GetPiece(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Review, "not reviewed yet")

	yes, no := true, false
	require.NoError(t, store.SaveReview(ctx, p.ID, sorter.PieceReview{PartCorrect: &yes, ColorCorrect: &no, ColorID: "5", Issues: []string{"blurry"}, ReviewedAt: time.Now()}))
	got, err = store.GetPiece(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Review)
	assert.True(t, *got.Review.PartCorrect)
	assert.False(t, *got.Review.ColorCorrect)
	assert.Equal(t, "5", got.Review.ColorID)
	assert.Equal(t, []string{"blurry"}, got.Review.Issues)

	// A review replaces the last one whole: the page always sends every answer.
	require.NoError(t, store.SaveReview(ctx, p.ID, sorter.PieceReview{PartCorrect: &no, ReviewedAt: time.Now()}))
	got, err = store.GetPiece(ctx, p.ID)
	require.NoError(t, err)
	assert.False(t, *got.Review.PartCorrect)
	assert.Nil(t, got.Review.ColorCorrect)
	assert.Empty(t, got.Review.Issues)

	assert.ErrorIs(t, store.SaveReview(ctx, "missing", sorter.PieceReview{ReviewedAt: time.Now()}), sorter.ErrPieceNotFound)
	assert.ErrorIs(t, sorter.PieceReview{Issues: []string{"sticky"}}.Check(), sorter.ErrBadReview)
}
