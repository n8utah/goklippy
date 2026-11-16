package models

import (
	"context"
	"testing"

	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/sorter/profile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Hive upload edited in place keeps its category ids, so bins keep their categories
// when carrying on; uploading the file again adds a second profile.
func TestRuleEditorEditHiveInPlace(t *testing.T) {
	svc, store := newSortingService(t)
	ctx := context.Background()
	hive, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	useProfile(t, svc, hive.ID)
	_, err = svc.Route(ctx, profile.Piece{Part: "3001", Color: "5"}, 0, true)
	require.NoError(t, err)

	ed, err := svc.EditableProfile(ctx, hive.ID)
	require.NoError(t, err)
	assert.True(t, ed.Converted)
	assert.True(t, ed.InBins)
	require.Len(t, ed.Document.Rules, 2)
	assert.Equal(t, "Bricks", ed.Document.Rules[0].Name)

	doc := ed.Document
	doc.Rules[0].Parts = append(doc.Rules[0].Parts, "3004")
	_, err = svc.SaveEditorProfile(ctx, hive.ID, doc, "")
	assert.ErrorIs(t, err, sorter.ErrProfileNeedsBins, "the bins' profile needs the bins choice")

	saved, err := svc.SaveEditorProfile(ctx, hive.ID, doc, "continue")
	require.NoError(t, err)
	assert.Equal(t, sorter.SourceEditor, saved.Source)
	bins, err := store.BinCategories(ctx, testCollectionID, hive.ID)
	require.NoError(t, err)
	assert.Equal(t, sorter.Assignments{{Layer: 1, Bin: 1}: {"r-bricks"}}, bins, "carrying on keeps the bins")
	res, err := svc.Route(ctx, profile.Piece{Part: "3004", Color: "5"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, "r-bricks", res.Category, "the edit routes straight away")
	assert.Equal(t, 1, res.Choice.Bin.Bin)

	again, err := svc.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	assert.NotEqual(t, hive.ID, again.ID, "a re-upload is a second profile")
	ed, err = svc.EditableProfile(ctx, hive.ID)
	require.NoError(t, err)
	assert.False(t, ed.Converted)
	assert.Contains(t, ed.Document.Rules[0].Parts, "3004", "the edited one is untouched")

	// Emptied: every bin starts afresh.
	_, err = svc.SaveEditorProfile(ctx, hive.ID, doc, "emptied")
	require.NoError(t, err)
	bins, err = store.BinCategories(ctx, testCollectionID, hive.ID)
	require.NoError(t, err)
	assert.Empty(t, bins)
}

func TestRuleEditorCopyStarterAndBuiltins(t *testing.T) {
	svc, store := newSortingService(t)
	ctx := context.Background()
	list, err := svc.Profiles(ctx)
	require.NoError(t, err)
	byType := list[0]

	_, err = svc.SaveEditorProfile(ctx, byType.ID, profile.Document{Name: "x"}, "")
	assert.ErrorIs(t, err, sorter.ErrBuiltinReadOnly)

	cp, err := svc.CopyProfile(ctx, byType.ID, "")
	require.NoError(t, err)
	assert.Equal(t, "By part type (copy)", cp.Name)
	assert.Equal(t, profile.LeftoverPartType, cp.Summary.Leftover)

	st, err := svc.CreateFromStarter(ctx, "sizes_then_types", "My sort")
	require.NoError(t, err)
	assert.Equal(t, "My sort", st.Name)
	assert.Len(t, st.Summary.Rules, 2)
	useProfile(t, svc, st.ID)

	require.NoError(t, svc.PreassignBins(ctx))
	bins, err := store.BinCategories(ctx, testCollectionID, st.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"rule-1/size:1x1"}, bins[sorter.BinKey{Layer: 1, Bin: 1}], "bricks by size first")

	res, err := svc.Route(ctx, profile.Piece{Part: "3001", Color: "5", PartCategory: "Brick", PartName: "Brick 2 x 4"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, "rule-1/size:2x4", res.Category)
	assert.Equal(t, "Bricks 2 x 4", res.CategoryName)

	doc, _ := profile.StarterDocument("sizes_then_types")
	check, err := svc.CheckPiece(ctx, doc, profile.Piece{Part: "3022", PartCategory: "Plate", PartName: "Plate 2 x 2"})
	require.NoError(t, err)
	assert.Equal(t, "Plates", check.RuleName)
	assert.Equal(t, "Plates 2 x 2", check.CategoryName)

	opts, err := svc.EditorOptions(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, opts.Presets)
	assert.Equal(t, "Brick", opts.Types[0].Value)
	assert.Equal(t, "1x1", opts.Sizes[0].Value)
	assert.NotEmpty(t, opts.Colors)
}

func TestRebrickableKeyStore(t *testing.T) {
	_, store := newSortingService(t)
	ctx := context.Background()
	key, err := store.RebrickableKey(ctx)
	require.NoError(t, err)
	assert.Empty(t, key)
	require.NoError(t, store.SaveRebrickableKey(ctx, "abc"))
	key, err = store.RebrickableKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, "abc", key)
}
