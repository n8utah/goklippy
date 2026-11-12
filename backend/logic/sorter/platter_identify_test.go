package sorter

import (
	"context"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChooseBurstWholeAndSharpestInOrder(t *testing.T) {
	photos := []burstPhoto{
		{sharp: 50, whole: true}, {sharp: 90, whole: false}, {sharp: 80, whole: true},
		{sharp: 10, whole: true}, {sharp: 70, whole: true},
	}
	assert.Equal(t, []int{0, 2, 4}, chooseBurst(photos, 3), "the whole ones, sharpest, in the order taken")
	assert.Equal(t, []int{0, 2, 3, 4}, chooseBurst(photos, 6), "never the cut-off one while whole ones remain")
	assert.Equal(t, []int{1}, chooseBurst([]burstPhoto{{sharp: 5}, {sharp: 9}}, 1), "none whole: the sharpest anyway")
}

func TestBestAttempt(t *testing.T) {
	named := func(label string, score float64) identifyAttempt {
		return identifyAttempt{label: label, res: brickognize.Result{Items: []brickognize.Item{{ID: label, Score: score}}}}
	}
	none := identifyAttempt{label: "none", res: brickognize.Result{}}
	assert.Equal(t, "b", bestAttempt([]identifyAttempt{named("a", 0.6), named("b", 0.8), none}).label)
	assert.Equal(t, "none", bestAttempt([]identifyAttempt{{label: "x", err: assert.AnError}, none}).label, "nothing named: the first that answered")
	assert.Error(t, bestAttempt([]identifyAttempt{{label: "x", err: assert.AnError}}).err)
}

func TestLeastAlikePicksDifferentViews(t *testing.T) {
	flat := make([]float64, thumbSide*thumbSide)
	ramp := make([]float64, thumbSide*thumbSide)
	for i := range ramp {
		ramp[i] = float64(i%thumbSide) - 7.5
	}
	photos := []burstPhoto{{thumb: flat}, {thumb: flat}, {thumb: ramp}, {thumb: flat}}
	assert.Equal(t, []int{2, 1}, leastAlike(photos, []int{0}, []int{1, 2, 3}, 2), "the different one first")
}

// fakeBrickognize names a part only when one of the photos is the given colour, and
// records each request's size.
type fakeBrickognize struct {
	mu    sync.Mutex
	sizes []int
	knows color.RGBA
}

func (f *fakeBrickognize) Identify(_ context.Context, jpegs [][]byte) (brickognize.Result, error) {
	f.mu.Lock()
	f.sizes = append(f.sizes, len(jpegs))
	f.mu.Unlock()
	for _, j := range jpegs {
		if centreIs(j, f.knows) {
			return brickognize.Result{Items: []brickognize.Item{{ID: "3001", Score: 0.9}}}, nil
		}
	}
	return brickognize.Result{}, nil
}

func centreIs(jpg []byte, c color.RGBA) bool {
	w := &SimWorld{landedC4: []SimPart{{RGB: c}}}
	_, ok := w.partInPhoto(jpg)
	return ok
}

// solidShots is n shots of a box filled with one colour each.
func solidShots(colors ...color.RGBA) []platterShot {
	var out []platterShot
	for i, c := range colors {
		img := image.NewRGBA(image.Rect(0, 0, 200, 200))
		for p := 0; p < len(img.Pix); p += 4 {
			img.Pix[p], img.Pix[p+1], img.Pix[p+2], img.Pix[p+3] = 128, 128, 128, 255
		}
		for y := 80; y < 120; y++ {
			for x := 80; x < 120; x++ {
				img.SetRGBA(x, y, c)
			}
		}
		out = append(out, platterShot{frame: img, box: vision.Box{X1: 80, Y1: 80, X2: 120, Y2: 120}, at: time.Unix(int64(i), 0)})
	}
	return out
}

func identifyTestPlatter(t *testing.T, id Identifier) *Platter {
	t.Helper()
	settings := func() MachineSettings { return DefaultMachineSettings() }
	return &Platter{deps: PlatterDeps{Identifier: id, Settings: settings, Records: NewRecords(nil, t.TempDir())}}
}

func TestIdentifySendsSeveralRequestsAtOnce(t *testing.T) {
	red := color.RGBA{200, 30, 30, 255}
	fake := &fakeBrickognize{knows: red}
	p := identifyTestPlatter(t, fake)
	ps := DefaultMachineSettings().Platter
	res := p.identify("p1", solidShots(red, red, red, red, red, red), ps)
	require.NoError(t, res.err)
	require.NotEmpty(t, res.result.Items)
	assert.ElementsMatch(t, []int{4, 1}, fake.sizes, "the 4 chosen together, and the most settled alone")
}

func TestIdentifyAddsTheUpstreamPictures(t *testing.T) {
	red, blue := color.RGBA{200, 30, 30, 255}, color.RGBA{30, 60, 200, 255}
	fake := &fakeBrickognize{knows: red}
	p := identifyTestPlatter(t, fake)
	upstream := solidShots(red)
	upJpg, err := upstream[0].crop()
	require.NoError(t, err)
	p.deps.Upstream = func(id string, n int) [][]byte { return [][]byte{upJpg} }
	res := p.identify("p1", solidShots(blue, blue, blue, blue), DefaultMachineSettings().Platter)
	assert.Equal(t, strategyUpstream, res.strategy, "C4 couldn't name it; with C3's picture it could")
	assert.Contains(t, fake.sizes, 5)
}

func TestIdentifyTriesOtherViewsWhenNothingNamedIt(t *testing.T) {
	red, blue := color.RGBA{200, 30, 30, 255}, color.RGBA{30, 60, 200, 255}
	fake := &fakeBrickognize{knows: red}
	p := identifyTestPlatter(t, fake)
	ps := DefaultMachineSettings().Platter
	ps.SendFrames = 2
	// Blue is sharper than nothing but all equal; the red view is a spare, unlike them.
	shots := solidShots(blue, blue, blue, red)
	res := p.identify("p1", shots, ps)
	require.NotEmpty(t, res.result.Items, "found on another view")
	assert.Contains(t, res.strategy, strategyOther)
}
