package sorter

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/vision"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestPlatterShotCropsTheFullSizeFrame(t *testing.T) {
	tracked := solid(320, 180, color.Black)
	var full bytes.Buffer
	require.NoError(t, jpeg.Encode(&full, solid(960, 540, color.White), nil))
	box := vision.Box{X1: 100, Y1: 60, X2: 140, Y2: 100} // 40×40 tracked pixels

	jpg, err := platterShot{frame: tracked, box: box, full: full.Bytes(), scale: 3}.crop()
	require.NoError(t, err)
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	require.NoError(t, err)
	// 40 px ×3 = 120, plus the crop margin each side.
	assert.Greater(t, img.Bounds().Dx(), 150, "cut from the 3× frame")
	r, _, _, _ := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
	assert.Greater(t, r, uint32(0xf000), "the full-size picture, not the tracked one")

	jpg, err = platterShot{frame: tracked, box: box}.crop()
	require.NoError(t, err)
	img, err = jpeg.Decode(bytes.NewReader(jpg))
	require.NoError(t, err)
	assert.Less(t, img.Bounds().Dx(), 80, "no full-size frame: the tracked one")

	jpg, err = platterShot{frame: tracked, box: box, full: []byte("not a jpeg"), scale: 3}.crop()
	require.NoError(t, err, "a broken full-size frame falls back")
	assert.NotEmpty(t, jpg)
}
