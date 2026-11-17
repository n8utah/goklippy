package sorter

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/vision"
)

// What lies past C4's ring (C3's gear beside it) is painted the channel's background in
// the photo, so Brickognize sees only the piece; inside the ring nothing changes.
func TestCropBlanksPastTheRing(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	ring := vision.Ring{Center: vision.Point{X: 100, Y: 100}, OuterRadius: 80}
	gear := color.RGBA{20, 20, 20, 255}
	channel := color.RGBA{190, 195, 200, 255}
	piece := color.RGBA{30, 60, 200, 255}
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			c := channel
			if dx, dy := float64(x)+0.5-100, float64(y)+0.5-100; dx*dx+dy*dy > 82*82 {
				c = gear
			} else if x >= 150 && x < 175 && y >= 90 && y < 110 {
				c = piece
			}
			img.SetRGBA(x, y, c)
		}
	}
	box := vision.Box{X1: 150, Y1: 90, X2: 175, Y2: 110}

	jpg, err := cropJPEG(img, box, ring)
	require.NoError(t, err)
	out, err := jpeg.Decode(bytes.NewReader(jpg))
	require.NoError(t, err)
	b := out.Bounds()
	near := func(c color.Color, want color.RGBA) bool {
		r, g, bl, _ := c.RGBA()
		d := func(a uint32, w uint8) int { return max(int(a>>8)-int(w), int(w)-int(a>>8)) }
		return d(r, want.R) < 12 && d(g, want.G) < 12 && d(bl, want.B) < 12
	}
	// The crop's right edge reaches past the ring: channel-coloured now, not the gear.
	assert.True(t, near(out.At(b.Max.X-1, b.Min.Y+b.Dy()/2), channel), "past the ring is background")
	assert.True(t, near(out.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2), piece), "the piece is untouched")

	// Without a ring the crop is as it was.
	jpg, err = cropJPEG(img, box, vision.Ring{})
	require.NoError(t, err)
	out, err = jpeg.Decode(bytes.NewReader(jpg))
	require.NoError(t, err)
	assert.True(t, near(out.At(out.Bounds().Max.X-1, out.Bounds().Min.Y+out.Bounds().Dy()/2), gear))
}

// CROP_FRAME=path/to/frame.jpg CROP_OUT=out.jpg: crops a real C4 frame (the box and ring
// below are from a piece that landed beside C3's gear) for a look.
func TestCropRealFrame(t *testing.T) {
	path, outPath := os.Getenv("CROP_FRAME"), os.Getenv("CROP_OUT")
	if path == "" || outPath == "" {
		t.Skip("set CROP_FRAME and CROP_OUT")
	}
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	frame, err := jpeg.Decode(f)
	require.NoError(t, err)
	z := vision.ChannelZones{FrameW: 640, FrameH: 480, Ring: vision.Ring{Center: vision.Point{X: 307, Y: 258}, InnerRadius: 40, OuterRadius: 276, ExitRadius: 191},
		Drop: vision.Span{Start: 68, End: 201}, Exit: vision.Span{Start: 24, End: 65}}
	b := frame.Bounds()
	ring := z.ScaledTo(b.Dx(), b.Dy()).Ring
	s := float64(b.Dx()) / 1280
	box := vision.Box{X1: int(136 * s), Y1: int(336 * s), X2: int(312 * s), Y2: int(524 * s)}
	jpg, err := cropJPEG(frame, box, ring)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outPath, jpg, 0o644))
}
