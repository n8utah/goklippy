package sorter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func firstPick(int) int { return 0 }

// tinyTower is two 12-bin layers, so the bins run out quickly.
func tinyTower() DistributorSettings {
	d := DefaultMachineSettings().Distributor
	d.Layers = []LayerSettings{{Bins: 12}, {Bins: 12}}
	return d
}

func TestChooseBinGivesNewCategoriesTheFirstEmptyBin(t *testing.T) {
	d := tinyTower()
	choice, cats := d.ChooseBin(Assignments{}, "bricks", 0, firstPick)
	require.NotNil(t, choice.Bin)
	assert.Equal(t, RouteNewBin, choice.Reason)
	assert.True(t, choice.Assigned)
	assert.Equal(t, BinKey{1, 1}, BinKey{choice.Bin.Layer, choice.Bin.Bin})
	assert.Equal(t, []string{"bricks"}, cats)

	assigned := Assignments{{1, 1}: {"bricks"}}
	choice, _ = d.ChooseBin(assigned, "plates", 0, firstPick)
	assert.Equal(t, BinKey{1, 2}, BinKey{choice.Bin.Layer, choice.Bin.Bin})

	choice, cats = d.ChooseBin(assigned, "bricks", 0, firstPick)
	assert.Equal(t, RouteAssignedBin, choice.Reason)
	assert.False(t, choice.Assigned)
	assert.Nil(t, cats)
	assert.Equal(t, BinKey{1, 1}, BinKey{choice.Bin.Layer, choice.Bin.Bin})
}

func TestChooseBinSpreadsACategoryOverItsBins(t *testing.T) {
	d := tinyTower()
	assigned := Assignments{{1, 3}: {"bricks"}, {2, 5}: {"bricks"}}
	var seen []int
	pick := func(n int) int {
		seen = append(seen, n)
		return n - 1
	}
	choice, _ := d.ChooseBin(assigned, "bricks", 0, pick)
	assert.Equal(t, []int{2}, seen, "chose among both bins")
	assert.Equal(t, BinKey{2, 5}, BinKey{choice.Bin.Layer, choice.Bin.Bin})
}

func TestChooseBinSkipsDisabledUnreachableAndSmall(t *testing.T) {
	d := tinyTower()
	d.Layers[0].DisabledSections = []int{1}
	choice, _ := d.ChooseBin(Assignments{}, "bricks", 0, firstPick)
	assert.Equal(t, BinKey{1, 3}, BinKey{choice.Bin.Layer, choice.Bin.Bin}, "section 1 is off")

	d.Layers[0].Disabled = true
	choice, _ = d.ChooseBin(Assignments{}, "bricks", 0, firstPick)
	assert.Equal(t, 2, choice.Bin.Layer, "layer 1 is off")

	d = tinyTower()
	d.Layers[0].MaxPieceMm = 30
	choice, _ = d.ChooseBin(Assignments{}, "bricks", 40, firstPick)
	assert.Equal(t, 2, choice.Bin.Layer, "40 mm is too long for layer 1")

	d = tinyTower()
	d.FirstOffset = 20
	d.MaxAngle = 300 // section 6 is past the travel limit
	assigned := Assignments{}
	for b := 1; b <= 10; b++ {
		assigned[BinKey{1, b}] = []string{"x"}
	}
	choice, _ = d.ChooseBin(assigned, "bricks", 0, firstPick)
	assert.Equal(t, BinKey{2, 1}, BinKey{choice.Bin.Layer, choice.Bin.Bin}, "layer 1's last section can't be reached")
}

func TestChooseBinBucketCases(t *testing.T) {
	d := tinyTower()
	choice, _ := d.ChooseBin(Assignments{}, "bricks", 90, firstPick)
	assert.Nil(t, choice.Bin)
	assert.Equal(t, RouteTooBig, choice.Reason)

	choice, _ = d.ChooseBin(Assignments{}, "misc", 0, firstPick)
	assert.Nil(t, choice.Bin, "misc never claims a bin")
	assert.Equal(t, RouteMisc, choice.Reason)

	full := Assignments{}
	for l := 1; l <= 2; l++ {
		for b := 1; b <= 12; b++ {
			full[BinKey{l, b}] = []string{"other"}
		}
	}
	choice, _ = d.ChooseBin(full, "bricks", 0, firstPick)
	assert.Nil(t, choice.Bin)
	assert.Equal(t, RouteNoBin, choice.Reason)

	full[BinKey{2, 12}] = []string{"misc"}
	choice, _ = d.ChooseBin(full, "bricks", 0, firstPick)
	require.NotNil(t, choice.Bin)
	assert.Equal(t, RouteMiscBin, choice.Reason)
	assert.Equal(t, BinKey{2, 12}, BinKey{choice.Bin.Layer, choice.Bin.Bin})
}

func TestChooseBinSharesWhenAllowed(t *testing.T) {
	d := tinyTower()
	d.SharedBins = true
	full := Assignments{}
	for l := 1; l <= 2; l++ {
		for b := 1; b <= 12; b++ {
			full[BinKey{l, b}] = []string{"a", "b"}
		}
	}
	full[BinKey{2, 4}] = []string{"a"}
	choice, cats := d.ChooseBin(full, "bricks", 0, firstPick)
	require.NotNil(t, choice.Bin)
	assert.Equal(t, RouteSharedBin, choice.Reason)
	assert.Equal(t, BinKey{2, 4}, BinKey{choice.Bin.Layer, choice.Bin.Bin}, "the bin with the fewest categories")
	assert.Equal(t, []string{"a", "bricks"}, cats)
	assert.Equal(t, []string{"a"}, full[BinKey{2, 4}], "the input isn't changed")
}

func TestLayerMaxPieceDefaults(t *testing.T) {
	assert.Equal(t, 50.0, LayerSettings{Bins: 18}.LayerMaxPieceMm())
	assert.Equal(t, 80.0, LayerSettings{Bins: 12}.LayerMaxPieceMm())
	assert.Equal(t, 35.0, LayerSettings{Bins: 18, MaxPieceMm: 35}.LayerMaxPieceMm())
}
