package sorter

import (
	"testing"

	"goklippy/backend/logic/sorter/profile"

	"github.com/stretchr/testify/assert"
)

func TestKitTracker(t *testing.T) {
	k := newKitTracker(map[string][]profile.KitLine{
		"kit": {{Part: "3001", Color: "5", Need: 2}, {Part: "3020", Need: 1}},
	})
	assert.False(t, k.full("kit", "3001", "5"))
	k.record("kit", "3001", "5")
	assert.False(t, k.full("kit", "3001", "5"))
	k.record("kit", "3001", "5")
	assert.True(t, k.full("kit", "3001", "5"), "two of two red bricks")
	assert.False(t, k.full("kit", "3001", "7"), "a blue brick isn't on the list, so never blocks")

	assert.False(t, k.full("kit", "3020", "11"))
	k.record("kit", "3020", "11")
	assert.True(t, k.full("kit", "3020", "1"), "an any-color line fills with any color")

	k.record("other", "3001", "5") // not a kit: ignored
	p := k.progress(func(c string) string { return c })
	assert.Equal(t, []KitProgress{{Category: "kit", Name: "kit", Need: 3, Found: 3}}, p)
}

func TestChooseBinSkipsFullBins(t *testing.T) {
	d := tinyTower()
	d.Layers[0].MaxPieces = 2
	assigned := Assignments{{1, 1}: {"bricks"}}
	choice, _ := d.ChooseBin(assigned, BinCounts{{1, 1}: 1}, "bricks", 0, firstPick)
	assert.Equal(t, 1, choice.Bin.Bin, "1 of 2: room left")

	choice, cats := d.ChooseBin(assigned, BinCounts{{1, 1}: 2}, "bricks", 0, firstPick)
	assert.Equal(t, RouteNewBin, choice.Reason, "full, so a new bin")
	assert.Equal(t, 2, choice.Bin.Bin)
	assert.Equal(t, []string{"bricks"}, cats)
}
