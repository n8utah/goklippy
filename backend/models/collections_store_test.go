package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/sorter/profile"
)

// testCollectionID is the first collection made in a fresh database.
const testCollectionID = 1

// newCollection makes a collection and returns its id.
func newCollection(t *testing.T, store *RecordsStore, name string) int64 {
	t.Helper()
	id, err := store.CreateCollection(context.Background(), name, nil, time.Now())
	require.NoError(t, err)
	return id
}

// pickCollection makes a collection and makes it the one svc sorts into.
func pickCollection(t *testing.T, db *sqlx.DB, svc *sorter.SortingService, name string) int64 {
	t.Helper()
	store := NewRecordsStore(db)
	id := newCollection(t, store, name)
	col, err := store.GetCollection(context.Background(), id)
	require.NoError(t, err)
	require.NoError(t, svc.SelectCollection(context.Background(), col))
	return id
}

// useProfile makes the current collection sort by the profile, bins not emptied.
func useProfile(t *testing.T, svc *sorter.SortingService, profileID int64) {
	t.Helper()
	col := svc.Collection()
	require.NotNil(t, col, "pick a collection first")
	require.NoError(t, svc.ChangeProfile(context.Background(), col.ID, profileID, false))
}

func TestCollectionsCreateRenameArchive(t *testing.T) {
	ctx := context.Background()
	store := NewRecordsStore(openTestDB(t))

	id := newCollection(t, store, "Sam's LEGO")
	_, err := store.CreateCollection(ctx, "Sam's LEGO", nil, time.Now())
	assert.ErrorIs(t, err, sorter.ErrCollectionName)
	other := newCollection(t, store, "Alex's LEGO")
	assert.ErrorIs(t, store.RenameCollection(ctx, other, "Sam's LEGO"), sorter.ErrCollectionName)
	require.NoError(t, store.RenameCollection(ctx, id, "Sam's bricks"))
	assert.ErrorIs(t, store.RenameCollection(ctx, 99, "x"), sorter.ErrCollectionNotFound)

	now := time.Now()
	require.NoError(t, store.ArchiveCollection(ctx, other, &now))
	cols, err := store.ListCollections(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 2)
	assert.Equal(t, "Sam's bricks", cols[0].Name, "unarchived first")
	assert.NotNil(t, cols[1].ArchivedAt)
	require.NoError(t, store.ArchiveCollection(ctx, other, nil))
	got, err := store.GetCollection(ctx, other)
	require.NoError(t, err)
	assert.Nil(t, got.ArchivedAt)
	_, err = store.GetCollection(ctx, 99)
	assert.ErrorIs(t, err, sorter.ErrCollectionNotFound)
}

// Pieces belong to a collection through their run; counts, filters and clearing the
// unknowns work per collection.
func TestCollectionsCountFilterAndClearUnknowns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	store := NewRecordsStore(db)
	dataDir := t.TempDir()
	records := sorter.NewRecords(store, dataDir)
	sam, alex := newCollection(t, store, "Sam"), newCollection(t, store, "Alex")
	t0 := time.Now().UTC().Truncate(time.Second)
	samRun1, err := store.StartRun(ctx, sam, nil, "", t0)
	require.NoError(t, err)
	samRun2, err := store.StartRun(ctx, sam, nil, "", t0.Add(time.Hour))
	require.NoError(t, err)
	alexRun, err := store.StartRun(ctx, alex, nil, "", t0.Add(2*time.Hour))
	require.NoError(t, err)

	save := func(runID int64, part string, status sorter.PieceStatus) string {
		t.Helper()
		p := sorter.Piece{ID: sorter.NewPieceID(), RunID: runID, Status: status, SeenAt: t0, PartID: part}
		require.NoError(t, store.SavePiece(ctx, p))
		return p.ID
	}
	save(samRun1, "3001", sorter.PieceDistributed)
	unknown := save(samRun1, "", sorter.PieceDistributed)
	save(samRun2, "3003", sorter.PieceDistributed)
	save(samRun2, "", sorter.PieceLost)
	save(samRun2, "", sorter.PieceIdentifying) // still on its way: not unknown
	save(alexRun, "", sorter.PieceDistributed)
	require.NoError(t, store.AddPieceImage(ctx, unknown, sorter.PieceImage{Path: "pieces/" + unknown[:2] + "/" + unknown + "/00.jpg"}))
	photo := filepath.Join(dataDir, "pieces", unknown[:2], unknown, "00.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(photo), 0o755))
	require.NoError(t, os.WriteFile(photo, []byte("jpeg"), 0o644))

	got, err := store.GetCollection(ctx, sam)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Runs)
	assert.Equal(t, 5, got.Pieces)
	assert.Equal(t, 2, got.Identified)
	assert.Equal(t, 2, got.Unknown)
	assert.NotNil(t, got.LastSortedAt)

	pieces, total, err := store.ListPieces(ctx, sorter.PieceFilter{CollectionID: sam, Unknown: true})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, pieces, 2)
	runs, err := store.ListRuns(ctx, alex, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "Alex", runs[0].CollectionName)

	cols, err := sorter.NewCollections(records, nil, func() int64 { return sam })
	require.NoError(t, err)
	_, err = cols.ClearUnknowns(ctx, sam)
	assert.ErrorIs(t, err, sorter.ErrCollectionSorting, "not while it's being sorted")

	cols, err = sorter.NewCollections(records, nil, func() int64 { return 0 })
	require.NoError(t, err)
	n, err := cols.ClearUnknowns(ctx, sam)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.NoFileExists(t, photo)
	got, err = store.GetCollection(ctx, sam)
	require.NoError(t, err)
	assert.Equal(t, 0, got.Unknown)
	assert.Equal(t, 3, got.Pieces, "identified and in-flight pieces stay")
	alexCol, err := store.GetCollection(ctx, alex)
	require.NoError(t, err)
	assert.Equal(t, 1, alexCol.Unknown, "other collections are left alone")
}

// Each collection has its own bins. Picking another collection empties the bins and
// starts its layout afresh, even one sorted before.
func TestCollectionsHaveTheirOwnBins(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	records := NewRecordsStore(db)
	sortingStore := NewSortingStore(db)
	svc := sorter.NewSortingService(ctx, sortingStore, testSettings)
	require.NoError(t, svc.AttachCounts(ctx, records))
	p, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)

	_, err = svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	assert.ErrorIs(t, err, sorter.ErrNoProfile, "no collection, so no profile to sort by")

	sam := pickCollection(t, db, svc, "Sam")
	useProfile(t, svc, p.ID)
	res, err := svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	require.NoError(t, err)
	brickBin := sorter.BinKey{Layer: res.Choice.Bin.Layer, Bin: res.Choice.Bin.Bin}
	run, err := records.StartRun(ctx, sam, &p.ID, p.Name, time.Now())
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, records.SavePiece(ctx, sorter.Piece{ID: sorter.NewPieceID(), RunID: run, Status: sorter.PieceDistributed,
		SeenAt: now, DistributedAt: &now, PartID: "3001", Category: res.Category, BinLayer: brickBin.Layer, BinNumber: brickBin.Bin}))
	svc.NoteDelivered(brickBin)

	cols, err := sorter.NewCollections(sorter.NewRecords(records, t.TempDir()), svc, func() int64 { return 0 })
	require.NoError(t, err)
	plan, err := cols.Plan(ctx, sam)
	require.NoError(t, err)
	require.NotNil(t, plan.Profile)
	require.Len(t, plan.Bins, 1)
	assert.Equal(t, "Bricks", plan.Bins[0].Categories[0].Name)
	assert.Equal(t, 1, plan.Bins[0].Pieces)

	// Alex: an empty layout, and the bins count from zero.
	time.Sleep(10 * time.Millisecond)
	pickCollection(t, db, svc, "Alex")
	assert.Equal(t, "Alex", svc.Bins().Collection.Name)
	for _, sec := range svc.Bins().Layers[0].Sections {
		for _, b := range sec.Bins {
			assert.Empty(t, b.Categories)
			assert.Zero(t, b.Count)
		}
	}

	// Back to Sam: the bins were emptied for Alex, so Sam starts afresh too.
	time.Sleep(10 * time.Millisecond)
	samCol, err := records.GetCollection(ctx, sam)
	require.NoError(t, err)
	require.NoError(t, svc.SelectCollection(ctx, samCol))
	bins, err := sortingStore.BinCategories(ctx, sam, p.ID)
	require.NoError(t, err)
	assert.Empty(t, bins)
	plan, err = cols.Plan(ctx, sam)
	require.NoError(t, err)
	assert.Empty(t, plan.Bins, "pieces from before its bins started afresh aren't in the plan")

	// The current collection survives a restart.
	svc2 := sorter.NewSortingService(ctx, sortingStore, testSettings)
	require.NotNil(t, svc2.Collection())
	assert.Equal(t, sam, svc2.Collection().ID)
}
