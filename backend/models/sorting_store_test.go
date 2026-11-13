package models

import (
	"context"
	"testing"

	"goklippy/backend/logic/sorter"

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

	_, err := svc.Route(ctx, "3001", "5", 0, true)
	assert.ErrorIs(t, err, sorter.ErrNoActiveProfile)

	p, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	assert.Equal(t, "Bricks and plates", p.Name)
	require.NoError(t, svc.Activate(ctx, p.ID, sorter.ActivateKeep))

	// Dry run: nothing saved.
	res, err := svc.Route(ctx, "3003", "5", 0, false)
	require.NoError(t, err)
	assert.Equal(t, "r-bricks", res.Category)
	assert.Equal(t, "Bricks", res.CategoryName)
	assert.Equal(t, sorter.RouteNewBin, res.Choice.Reason)
	saved, err := store.BinCategories(ctx, testCollectionID, p.ID)
	require.NoError(t, err)
	assert.Empty(t, saved)

	// Committed: bricks get bin 1, plates bin 2, and bricks keep bin 1.
	res, err = svc.Route(ctx, "3003", "5", 0, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Choice.Bin.Bin)
	res, err = svc.Route(ctx, "3023", "1", 0, true)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Choice.Bin.Bin)
	res, err = svc.Route(ctx, "3001pb01", "7", 0, true) // a printed brick
	require.NoError(t, err)
	assert.Equal(t, sorter.RouteAssignedBin, res.Choice.Reason)
	assert.Equal(t, 1, res.Choice.Bin.Bin)

	res, err = svc.Route(ctx, "9999", "7", 0, true)
	require.NoError(t, err)
	assert.Equal(t, "misc", res.Category)
	assert.Nil(t, res.Choice.Bin, "misc goes to the bucket")

	saved, err = store.BinCategories(ctx, testCollectionID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, sorter.Assignments{{Layer: 1, Bin: 1}: {"r-bricks"}, {Layer: 1, Bin: 2}: {"r-plates"}}, saved)

	// A restart picks the active profile and its bins back up.
	svc2 := sorter.NewSortingService(ctx, store, testSettings)
	res, err = svc2.Route(ctx, "3020", "1", 0, false)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Choice.Bin.Bin)
}

func TestSortingProfilesKeepTheirOwnBins(t *testing.T) {
	svc, _ := newSortingService(t)
	ctx := context.Background()
	bricks, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	other, err := svc.Import(ctx, []byte(otherProfile))
	require.NoError(t, err)

	require.NoError(t, svc.Activate(ctx, bricks.ID, sorter.ActivatePreassign))
	view := svc.Bins()
	require.NotNil(t, view.Profile)
	assert.Equal(t, bricks.ID, view.Profile.ID)
	bin1 := view.Layers[0].Sections[0].Bins[0]
	assert.Equal(t, []sorter.CategoryRef{{ID: "r-bricks", Name: "Bricks"}}, bin1.Categories)
	assert.Equal(t, []sorter.CategoryRef{{ID: "r-plates", Name: "Plates"}}, view.Layers[0].Sections[0].Bins[1].Categories)

	require.NoError(t, svc.Activate(ctx, other.ID, sorter.ActivateKeep))
	assert.Empty(t, svc.Bins().Layers[0].Sections[0].Bins[0].Categories, "the other profile starts unassigned")

	require.NoError(t, svc.Activate(ctx, bricks.ID, sorter.ActivateKeep))
	assert.Equal(t, "r-bricks", svc.Bins().Layers[0].Sections[0].Bins[0].Categories[0].ID, "back to its own bins")

	require.NoError(t, svc.Activate(ctx, bricks.ID, sorter.ActivateReset))
	assert.Empty(t, svc.Bins().Layers[0].Sections[0].Bins[0].Categories)

	assert.ErrorIs(t, svc.Delete(ctx, bricks.ID), sorter.ErrProfileActive)
	require.NoError(t, svc.Delete(ctx, other.ID))
	assert.ErrorIs(t, svc.Delete(ctx, other.ID), sorter.ErrProfileNotFound)
	list, err := svc.Profiles(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Active)
	assert.Equal(t, "program", list[0].Summary.Format)

	assert.ErrorIs(t, svc.Activate(ctx, bricks.ID, sorter.Activation("bogus")), sorter.ErrInvalidActivation)
}

func TestSortingSetBinMovesCategories(t *testing.T) {
	svc, _ := newSortingService(t)
	ctx := context.Background()
	p, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	require.NoError(t, svc.Activate(ctx, p.ID, sorter.ActivatePreassign))

	// Bricks move from bin 1 to layer 2 bin 3, joined by misc.
	require.NoError(t, svc.SetBin(ctx, sorter.BinKey{Layer: 2, Bin: 3}, []string{"r-bricks", "misc", "r-bricks"}))
	view := svc.Bins()
	assert.Empty(t, view.Layers[0].Sections[0].Bins[0].Categories)
	got := view.Layers[1].Sections[1].Bins[0] // layer 2, section 2, first bin = bin 3
	assert.Equal(t, 3, got.Bin)
	assert.Equal(t, []sorter.CategoryRef{{ID: "r-bricks", Name: "Bricks"}, {ID: "misc", Name: "Misc"}}, got.Categories)

	res, err := svc.Route(ctx, "9999", "7", 0, false)
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
	require.NoError(t, svc.Activate(ctx, p.ID, sorter.ActivateKeep))

	res, err := svc.Route(ctx, "3001", "7", 0, true)
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
