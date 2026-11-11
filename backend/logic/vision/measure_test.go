package vision

import (
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bowlFrame draws a bright bowl (rim at rc, radius rr) on a dark background, shaded
// darker on one side, with a dark hole at hc (radius hr) and a dark bracket reaching
// in from the top, like C3's camera sees it.
func bowlFrame(w, h int, rc Point, rr float64, hc Point, hr float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(12) // the dark surroundings
			d := math.Hypot(float64(x)-rc.X, float64(y)-rc.Y)
			if d <= rr {
				// Bright, with a shadow falling across the lower right.
				v = uint8(200 - 90*math.Max(0, (float64(x)-rc.X)/rr)*math.Max(0, (float64(y)-rc.Y)/rr))
				if math.Hypot(float64(x)-hc.X, float64(y)-hc.Y) <= hr {
					v = 20
				}
				if x > int(rc.X)-25 && x < int(rc.X)+25 && float64(y) < rc.Y-hr-10 {
					v = 25 // the bracket
				}
			}
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func TestMeasureRingSynthetic(t *testing.T) {
	rc, hc := Point{350, 216}, Point{356, 232}
	img := bowlFrame(640, 480, rc, 272, hc, 55)
	m, err := MeasureRing(img)
	require.NoError(t, err)
	assert.InDelta(t, rc.X, m.RimCenter.X, 2)
	assert.InDelta(t, rc.Y, m.RimCenter.Y, 2)
	assert.InDelta(t, 272, m.RimRadius, 2)
	assert.InDelta(t, hc.X, m.HoleCenter.X, 3)
	assert.InDelta(t, hc.Y, m.HoleCenter.Y, 3)
	assert.InDelta(t, 55, m.HoleRadius, 3)
	assert.Greater(t, m.OutOfFrame, 0.1, "the rim runs off the bottom and sides")
	// The ring: inside the hole's far side, out a little past the rim.
	assert.InDelta(t, math.Hypot(6, 16)+55+innerMargin, m.Ring.InnerRadius, 3)
	assert.InDelta(t, 272*outerGrow, m.Ring.OuterRadius, 2)
}

func TestMeasureRingWholeBowlInView(t *testing.T) {
	m, err := MeasureRing(bowlFrame(640, 480, Point{320, 240}, 200, Point{320, 240}, 40))
	require.NoError(t, err)
	assert.InDelta(t, 200, m.RimRadius, 2)
	assert.Zero(t, m.OutOfFrame)
}

func TestMeasureRingNoBowl(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for i := range img.Pix {
		img.Pix[i] = 90
	}
	_, err := MeasureRing(img)
	assert.ErrorIs(t, err, ErrNoRing)
}

// TestMeasureRingC3 is C3 on the machine (2026-11-11, dim, light not working). Checked
// by eye on the frame: the rim is at about (367, 227), radius 292, following the
// bowl's dark edge all round (a first fit by hand, from the left edge only with a
// brighter split, came out at (350, 216) r 272, inside the shadowed side), and the
// hole at about (354, 225), radius 49.
func TestMeasureRingC3(t *testing.T) {
	f, err := os.Open("testdata/c3-empty-640.jpg")
	require.NoError(t, err)
	defer f.Close()
	img, err := jpeg.Decode(f)
	require.NoError(t, err)
	m, err := MeasureRing(img)
	require.NoError(t, err)
	t.Logf("rim (%.1f, %.1f) r %.1f fit %.2f; hole (%.1f, %.1f) r %.1f fit %.2f; out of frame %.0f%%; ring %+v",
		m.RimCenter.X, m.RimCenter.Y, m.RimRadius, m.RimFit, m.HoleCenter.X, m.HoleCenter.Y, m.HoleRadius, m.HoleFit, m.OutOfFrame*100, m.Ring)
	assert.InDelta(t, 367, m.RimCenter.X, 5)
	assert.InDelta(t, 227, m.RimCenter.Y, 5)
	assert.InDelta(t, 292, m.RimRadius, 5)
	assert.InDelta(t, 354, m.HoleCenter.X, 4)
	assert.InDelta(t, 225, m.HoleCenter.Y, 4)
	assert.InDelta(t, 49, m.HoleRadius, 4)
	assert.Greater(t, m.RimFit, 0.8)
	assert.Greater(t, m.OutOfFrame, 0.4, "half of C3's rim is outside this camera's view")
}
