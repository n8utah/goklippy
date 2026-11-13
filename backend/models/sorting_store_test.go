package models

import (
	"context"
	"testing"

	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/sorter/profile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bricksProfile = `{
  "name": "Bricks and plates", "schema_version": 1,
  "rules": [{"id": "r-bricks", "name": "Bricks"}, {"id": "r-plates", "name": "Plates"}],
  "categories": {"r-bricks": {"name": "Bricks"}, "r-plates": {"name": "Plates"}},
  "program": {
    "rules": [
      {"category": "r-bricks", "parts": ["3001", "3003"], "colors": null},
      {"category": "r-plates", "parts": ["3020", "3023"], "colors": null}
    ],
    "default": "misc"
  }
}`

const otherProfile = `{"name": "Other", "part_to_category": {"any_color-3001": "everything"}, "default_category_id": "everything"}`

// A small tower: two 12-bin layers.
func testSettings() sorter.MachineSettings {
	s := sorter.DefaultMachineSettings()
	s.Distributor.Layers = []sorter.LayerSettings{{Bins: 12}, {Bins: 12}}
	return s
}

func newSortingService(t *testing.T) (*sorter.SortingService, *SortingStore) {
	t.Helper()
	db := openTestDB(t)
	store := NewSortingStore(db)
	svc := sorter.NewSortingService(context.Background(), store, testSettings)
	pickCollection(t, db, svc, "Test")
	return svc, store
}

func TestSortingRoutesAndRemembersBins(t *testing.T) {
	svc, store := newSortingService(t)
	ctx := context.Background()

	_, err := svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	assert.ErrorIs(t, err, sorter.ErrNoProfile)

	p, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	assert.Equal(t, "Bricks and plates", p.Name)
	useProfile(t, svc, p.ID)

	// Dry run: nothing saved.
	res, err := svc.Route(ctx, profile.Piece{Part: "3003", Color: "5"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, "r-bricks", res.Category)
	assert.Equal(t, "Bricks", res.CategoryName)
	assert.Equal(t, sorter.RouteNewBin, res.Choice.Reason)
	saved, err := store.BinCategories(ctx, testCollectionID, p.ID)
	require.NoError(t, err)
	assert.Empty(t, saved)

	// Committed: bricks get bin 1, plates bin 2, and bricks keep bin 1.
	res, err = svc.Route(ctx, profile.Piece{Part: "3003", Color: "5"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Choice.Bin.Bin)
	res, err = svc.Route(ctx, profile.Piece{Part: "3023", Color: "1"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Choice.Bin.Bin)
	res, err = svc.Route(ctx, profile.Piece{Part: "3001pb01", Color: "7"}, 0, true) // a printed brick
	require.NoError(t, err)
	assert.Equal(t, sorter.RouteAssignedBin, res.Choice.Reason)
	assert.Equal(t, 1, res.Choice.Bin.Bin)

	res, err = svc.Route(ctx, profile.Piece{Part: "9999", Color: "7"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "misc", res.Category)
	assert.Nil(t, res.Choice.Bin, "misc goes to the bucket")

	saved, err = store.BinCategories(ctx, testCollectionID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, sorter.Assignments{{Layer: 1, Bin: 1}: {"r-bricks"}, {Layer: 1, Bin: 2}: {"r-plates"}}, saved)

	// A restart picks the collection's profile and its bins back up.
	svc2 := sorter.NewSortingService(ctx, store, testSettings)
	res, err = svc2.Route(ctx, profile.Piece{Part: "3020", Color: "1"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Choice.Bin.Bin)
}

// The built-ins are always there, first, and can't be deleted; an upload a collection
// sorts by can't be deleted either.
func TestSortingProfilesListAndDelete(t *testing.T) {
	svc, store := newSortingService(t)
	ctx := context.Background()
	bricks, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	other, err := svc.Import(ctx, []byte(otherProfile))
	require.NoError(t, err)
	useProfile(t, svc, bricks.ID)

	list, err := svc.Profiles(ctx)
	require.NoError(t, err)
	require.Len(t, list, 4)
	assert.Equal(t, "By part type", list[0].Name)
	assert.Equal(t, sorter.SourceBuiltin, list[0].Source)
	assert.Equal(t, "By colour", list[1].Name)
	assert.Equal(t, "Other", list[2].Name, "then uploads, newest first")
	assert.Equal(t, []sorter.CollectionRef{{ID: testCollectionID, Name: "Test"}}, list[3].UsedBy)
	assert.Empty(t, list[2].UsedBy)

	assert.ErrorIs(t, svc.Delete(ctx, list[0].ID), sorter.ErrProfileBuiltin)
	assert.ErrorIs(t, svc.Delete(ctx, bricks.ID), sorter.ErrProfileInUse)
	require.NoError(t, svc.Delete(ctx, other.ID))
	assert.ErrorIs(t, svc.Delete(ctx, other.ID), sorter.ErrProfileNotFound)

	// A restart doesn't add the built-ins again.
	sorter.NewSortingService(ctx, store, testSettings)
	list, err = svc.Profiles(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

// Changing the current collection's profile: emptied starts every bin afresh; not
// emptied keeps the bins holding pieces out of the new profile's layout.
func TestSortingChangeProfileKeepsFullBinsOut(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	records := NewRecordsStore(db)
	svc := sorter.NewSortingService(ctx, NewSortingStore(db), testSettings)
	require.NoError(t, svc.AttachCounts(ctx, records))
	col := pickCollection(t, db, svc, "Sam")
	bricks, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	other, err := svc.Import(ctx, []byte(otherProfile))
	require.NoError(t, err)

	useProfile(t, svc, bricks.ID)
	require.NoError(t, svc.PreassignBins(ctx))
	// Bricks in bin 1 (two pieces); plates were given bin 2 but none arrived.
	svc.NoteDelivered(sorter.BinKey{Layer: 1, Bin: 1})
	svc.NoteDelivered(sorter.BinKey{Layer: 1, Bin: 1})

	// Not emptied: a fresh layout, and the "everything" category skips bin 1.
	require.NoError(t, svc.ChangeProfile(ctx, col, other.ID, false))
	assert.Empty(t, svc.Bins().Layers[0].Sections[0].Bins[0].Categories)
	res, err := svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "everything", res.Category)
	assert.Equal(t, 2, res.Choice.Bin.Bin, "bin 1 still holds bricks")
	assert.Equal(t, 2, svc.Bins().Layers[0].Sections[0].Bins[0].Count, "its pieces are still counted")

	// Pre-assigning skips it too.
	useProfile(t, svc, bricks.ID)
	require.NoError(t, svc.PreassignBins(ctx))
	bins := svc.Bins().Layers[0].Sections[0].Bins
	assert.Empty(t, bins[0].Categories)
	assert.Equal(t, "r-bricks", bins[1].Categories[0].ID)

	// Emptied: every bin counts from zero and is free again.
	require.NoError(t, svc.ChangeProfile(ctx, col, other.ID, true))
	bins = svc.Bins().Layers[0].Sections[0].Bins
	assert.Zero(t, bins[0].Count)
	assert.Empty(t, bins[1].Categories)
	res, err = svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Choice.Bin.Bin)

	got, err := records.GetCollection(ctx, col)
	require.NoError(t, err)
	require.NotNil(t, got.ProfileID)
	assert.Equal(t, other.ID, *got.ProfileID)
	assert.Equal(t, "Other", got.ProfileName)

	// Another collection's profile is only recorded; the current one is untouched.
	alex := newCollection(t, records, "Alex")
	require.NoError(t, svc.ChangeProfile(ctx, alex, bricks.ID, false))
	assert.Equal(t, other.ID, svc.Active().ID)
	assert.ErrorIs(t, svc.ChangeProfile(ctx, alex, 999, false), sorter.ErrProfileNotFound)
}

// Sorting by part type and by colour with the built-ins.
func TestSortingBuiltinProfiles(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO catalog_parts (part_id, color_id, color_name, first_seen, last_seen) VALUES ('3001', '11', 'Black', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	svc := sorter.NewSortingService(ctx, NewSortingStore(db), testSettings)
	pickCollection(t, db, svc, "Test")
	list, err := svc.Profiles(ctx)
	require.NoError(t, err)
	partType, color := list[0], list[1]

	useProfile(t, svc, partType.ID)
	res, err := svc.Route(ctx, profile.Piece{Part: "3710", Color: "5", PartCategory: "Plate, Round"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "Plate", res.Category)
	assert.Equal(t, 1, res.Choice.Bin.Bin)
	res, err = svc.Route(ctx, profile.Piece{Part: "3023", Color: "5", PartCategory: "Plate"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Choice.Bin.Bin, "every plate in one bin")
	res, err = svc.Route(ctx, profile.Piece{Part: "x1", Color: "5"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "misc", res.Category, "no part type")
	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 1, Bin: 5}, []string{"Tile", "Wedge"}), "two types in one bin")
	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 1, Bin: 6}, []string{"Bionicle"}), "any part type")
	require.NoError(t, svc.PreassignBins(ctx))
	assert.Equal(t, "Brick", svc.Bins().Layers[0].Sections[0].Bins[0].Categories[0].ID, "pre-assigned in the usual order")

	useProfile(t, svc, color.ID)
	res, err = svc.Route(ctx, profile.Piece{Part: "3001", Color: "5", ColorName: "Red"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "color_5", res.Category)
	assert.Equal(t, "Red", res.CategoryName, "named from the piece")
	view := svc.Bins()
	assert.Contains(t, view.Categories, sorter.CategoryRef{ID: "color_11", Name: "Black"}, "colours from the catalog are offered")
	assert.Contains(t, view.Categories, sorter.CategoryRef{ID: "color_5", Name: "Red"})
	assert.ErrorIs(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 1}, []string{"Plate"}), sorter.ErrUnknownCategory)
}

func TestSortingSetBinMovesCategories(t *testing.T) {
	svc, _ := newSortingService(t)
	ctx := context.Background()
	p, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	useProfile(t, svc, p.ID)
	require.NoError(t, svc.PreassignBins(ctx))

	// Bricks move from bin 1 to layer 2 bin 3, joined by misc.
	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 3}, []string{"r-bricks", "misc", "r-bricks"}))
	view := svc.Bins()
	assert.Empty(t, view.Layers[0].Sections[0].Bins[0].Categories)
	got := view.Layers[1].Sections[1].Bins[0] // layer 2, section 2, first bin = bin 3
	assert.Equal(t, 3, got.Bin)
	assert.Equal(t, []sorter.CategoryRef{{ID: "r-bricks", Name: "Bricks"}, {ID: "misc", Name: "Misc"}}, got.Categories)

	res, err := svc.Route(ctx, profile.Piece{Part: "9999", Color: "7"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, sorter.RouteMiscBin, res.Choice.Reason, "misc now has a bin")

	assert.ErrorIs(t, svc.SetBin(ctx, sorter.BinKey{Layer: 1, Bin: 1}, []string{"nope"}), sorter.ErrUnknownCategory)
	assert.ErrorIs(t, svc.SetBin(ctx, sorter.BinKey{Layer: 1, Bin: 40}, nil), sorter.ErrNoSuchBin)

	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 3}, nil))
	assert.Empty(t, svc.Bins().Layers[1].Sections[1].Bins[0].Categories)

	require.NoError(t, svc.ResetBins(ctx))
	assert.Empty(t, svc.Bins().Layers[0].Sections[0].Bins[1].Categories)
}

func TestSortingColorFallbackCategoriesCanBeMoved(t *testing.T) {
	svc, _ := newSortingService(t)
	ctx := context.Background()
	p, err := svc.Import(ctx, []byte(`{"name": "By color", "program": {"rules": [], "fallback": {"by": "color"}}}`))
	require.NoError(t, err)
	useProfile(t, svc, p.ID)

	res, err := svc.Route(ctx, profile.Piece{Part: "3001", Color: "7"}, 0, true)
	require.NoError(t, err)
	assert.Equal(t, "color_7", res.Category)
	assert.Contains(t, svc.Bins().Categories, sorter.CategoryRef{ID: "color_7", Name: "Color 7"}, "listed once a bin has it")

	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 1}, []string{"color_7"}), "an in-use category can move")
	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 2}, []string{"color_11"}), "any color, when sorting by color")
	assert.ErrorIs(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 3}, []string{"bricks"}), sorter.ErrUnknownCategory)
}

func TestSortingImportRejectsBadFiles(t *testing.T) {
	svc, _ := newSortingService(t)
	_, err := svc.Import(context.Background(), []byte(`{"name": "x"}`))
	assert.Error(t, err)
}
