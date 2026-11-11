package vision

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rotorFrame is the empty channel with its rotor turned to angle: 12 light and dark
// facets around the center, like the cone, plus a fixed dark bracket that doesn't turn.
func rotorFrame(angle float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			a := math.Atan2(float64(y-240), float64(x-320))*180/math.Pi - angle
			facet := int(math.Floor(NormalizeAngle(a)/30)) % 2
			v := uint8(150)
			if facet == 1 {
				v = 220
			}
			if x > 300 && x < 340 && y < 120 { // the bracket
				v = 30
			}
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func withPiece(img *image.RGBA, box Box) *image.RGBA {
	for y := box.Y1; y < box.Y2; y++ {
		for x := box.X1; x < box.X2; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 30, 30, 255})
		}
	}
	return img
}

func TestViewsCancelATurningPattern(t *testing.T) {
	z := testZones()
	frame := withPiece(rotorFrame(37), at(230, 20)) // on the travel path (200°–350°)

	// One picture taken at 0°: at 37° the facet edges all show up as pieces.
	single := NewBackgroundDetector()
	single.Learn = 0
	single.SetBackground(rotorFrame(0))
	boxes, err := single.Detect(frame, z)
	require.NoError(t, err)
	assert.Greater(t, len(boxes), 1, "the facets look like pieces")

	// Views every 2° all the way round: only the piece.
	var views []image.Image
	for a := 0.0; a < 360; a += 2 {
		views = append(views, rotorFrame(a))
	}
	d := NewBackgroundDetector()
	require.NoError(t, d.SetViews(views, 2))
	assert.Equal(t, 180, d.Views())
	assert.True(t, d.HasBackground())
	boxes, err = d.Detect(frame, z)
	require.NoError(t, err)
	require.Len(t, boxes, 1, "only the piece")
	want := at(230, 20)
	assert.InDelta(t, want.Center().X, boxes[0].Center().X, 3)
	assert.InDelta(t, want.Center().Y, boxes[0].Center().Y, 3)
	// The facets repeat every 60°, so 36°, 96°, … match equally well as 37°'s view.
	assert.Equal(t, 36.0, math.Mod(float64(d.LastView())*2, 60))
}

func TestViewsOfOneRepeatCoverEveryAngle(t *testing.T) {
	// The synthetic rotor repeats every 60°: views across 0–58° are enough for a
	// frame at 217°.
	var views []image.Image
	for a := 0.0; a < 60; a += 2 {
		views = append(views, rotorFrame(a))
	}
	d := NewBackgroundDetector()
	require.NoError(t, d.SetViews(views, 2))
	boxes, err := d.Detect(withPiece(rotorFrame(217), at(300, 20)), testZones())
	require.NoError(t, err)
	require.Len(t, boxes, 1, "only the piece")
	assert.Equal(t, 18, d.LastView(), "217° looks like 36°")
}

func TestSingleBackgroundReplacesViews(t *testing.T) {
	d := NewBackgroundDetector()
	require.NoError(t, d.SetViews([]image.Image{rotorFrame(0), rotorFrame(2)}, 2))
	d.SetBackground(rotorFrame(0))
	assert.Zero(t, d.Views())
	assert.Equal(t, -1, d.LastView())
	assert.True(t, d.HasBackground())
}

func TestViewsMustMatchTheFrameSize(t *testing.T) {
	d := NewBackgroundDetector()
	small := image.NewRGBA(image.Rect(0, 0, 320, 240))
	assert.Error(t, d.SetViews([]image.Image{rotorFrame(0), small}, 2))
	require.NoError(t, d.SetViews([]image.Image{small}, 2))
	_, err := d.Detect(rotorFrame(0), testZones())
	assert.Error(t, err)
}
