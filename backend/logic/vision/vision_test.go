package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A 640×480 camera over a channel centered in the frame. Pieces land between 200°
// and 260°, meter through a 15° precise zone, and leave between 330° and 350°.
func testZones() ChannelZones {
	return ChannelZones{
		FrameW: 640, FrameH: 480,
		Ring:         Ring{Center: Point{320, 240}, InnerRadius: 100, OuterRadius: 200, ExitRadius: 190},
		Drop:         Span{Start: 200, End: 260},
		Exit:         Span{Start: 330, End: 350},
		PreciseWidth: 15,
	}
}

// at is the box of a size-pixel square centered at angle on the ring, radius 150.
func at(angle float64, size int) Box {
	rad := angle * math.Pi / 180
	cx, cy := 320+150*math.Cos(rad), 240+150*math.Sin(rad)
	h := size / 2
	return Box{int(cx) - h, int(cy) - h, int(cx) + h, int(cy) + h}
}

func blankFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 128, 128, 128, 255
	}
	return img
}

func paint(img *image.RGBA, b Box, c color.RGBA) {
	for y := b.Y1; y < b.Y2; y++ {
		for x := b.X1; x < b.X2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

var red = color.RGBA{220, 30, 30, 255}

func TestSpans(t *testing.T) {
	s := Span{Start: 350, End: 10}
	assert.InDelta(t, 20, s.Width(), 1e-9)
	assert.True(t, s.Contains(355))
	assert.True(t, s.Contains(5))
	assert.False(t, s.Contains(20))
	assert.InDelta(t, 30, ForwardDeg(350, 20), 1e-9)
}

func TestZones(t *testing.T) {
	z := testZones()
	assert.Equal(t, ZoneDrop, z.ZoneAt(230))
	assert.Equal(t, ZonePrecise, z.ZoneAt(320))
	assert.Equal(t, ZoneExit, z.ZoneAt(340))
	assert.Equal(t, ZoneNone, z.ZoneAt(290))
	assert.InDelta(t, 100, z.ForwardToExit(230), 1e-9)
	assert.InDelta(t, -5, z.ForwardToExit(335), 1e-9)

	assert.True(t, z.OnChannel(at(230, 0).Center()))
	assert.False(t, z.OnChannel(Point{320, 240}), "the hub")
	assert.False(t, z.OnChannel(at(100, 0).Center()), "outside the travel arc")

	scaled := z.ScaledTo(1280, 960)
	assert.InDelta(t, 640, scaled.Ring.Center.X, 1e-9)
	assert.InDelta(t, 400, scaled.Ring.OuterRadius, 1e-9)
}

func TestObserve(t *testing.T) {
	z := testZones()
	now := time.Now()
	obs := Observe(z, []Box{at(230, 20), at(338, 20), at(100, 20)}, 4, now)
	assert.True(t, obs.Valid)
	assert.True(t, obs.InDrop)
	assert.True(t, obs.InExit)
	require.Len(t, obs.Pieces, 2, "the box off the channel is dropped")
	assert.Equal(t, ZoneExit, obs.Pieces[0].Zone, "leading piece first")
	assert.Equal(t, ZoneDrop, obs.Pieces[1].Zone)

	// Two halves of one two-color brick become one piece.
	a, b := at(230, 20), at(230, 20)
	a.X2 = (a.X1 + a.X2) / 2
	b.X1 = a.X2 + 2
	obs = Observe(z, []Box{a, b}, 4, now)
	require.Len(t, obs.Pieces, 1)
	require.Len(t, obs.Merged, 1)
	assert.Len(t, obs.Merged[0].Parts, 2)

	empty := Observe(z, nil, 4, now)
	assert.True(t, empty.Empty())
	assert.False(t, Observation{}.Empty(), "no frame yet isn't empty")
}

func TestBackgroundDetector(t *testing.T) {
	z := testZones()
	d := NewBackgroundDetector()
	_, err := d.Detect(blankFrame(), z)
	assert.ErrorIs(t, err, ErrNoBackground)

	d.SetBackground(blankFrame())
	frame := blankFrame()
	paint(frame, at(230, 24), red)
	paint(frame, at(300, 24), color.RGBA{30, 30, 220, 255})
	paint(frame, Box{10, 10, 40, 40}, red) // off the channel
	paint(frame, at(250, 4), red)          // too small: noise
	boxes, err := d.Detect(frame, z)
	require.NoError(t, err)
	require.Len(t, boxes, 2)

	obs := Observe(z, boxes, 4, time.Now())
	require.Len(t, obs.Pieces, 2)
	assert.InDelta(t, 300, obs.Pieces[0].Angle, 3)
	assert.InDelta(t, 230, obs.Pieces[1].Angle, 3)

	got := MeanColor(frame, at(230, 20), 1)
	assert.InDelta(t, 220.0/255, got[0], 0.01)

	boxes, err = d.Detect(blankFrame(), z)
	require.NoError(t, err)
	assert.Empty(t, boxes)
}

func TestTrackerFollowsPiecesAlongTheArc(t *testing.T) {
	z := testZones()
	tr := NewTracker()
	start := time.Now()

	// Two pieces, 40° apart, both moving 5° per 100 ms frame.
	var ids []int
	for i := 0; i < 12; i++ {
		now := start.Add(time.Duration(i) * 100 * time.Millisecond)
		obs := Observe(z, []Box{at(210+float64(i)*5, 20), at(250+float64(i)*5, 20)}, 4, now)
		colors := [][3]float64{{0.9, 0.1, 0.1}, {0.1, 0.1, 0.9}}
		if obs.Pieces[0].Box != at(250+float64(i)*5, 20) {
			colors[0], colors[1] = colors[1], colors[0]
		}
		tr.Update(z, &obs, colors)
		if i == 0 {
			ids = []int{obs.Pieces[0].TrackID, obs.Pieces[1].TrackID}
			assert.NotEqual(t, ids[0], ids[1])
			continue
		}
		assert.Equal(t, ids, []int{obs.Pieces[0].TrackID, obs.Pieces[1].TrackID}, "frame %d", i)
	}

	// A piece unseen for longer than the coast time comes back with a new id.
	late := Observe(z, []Box{at(330, 20)}, 4, start.Add(10*time.Second))
	tr.Update(z, &late, nil)
	assert.NotContains(t, ids, late.Pieces[0].TrackID)
}

func TestTrackerColorGateKeepsIdsApart(t *testing.T) {
	z := testZones()
	tr := NewTracker()
	now := time.Now()
	obs := Observe(z, []Box{at(230, 20)}, 4, now)
	tr.Update(z, &obs, [][3]float64{{0.9, 0.1, 0.1}})
	first := obs.Pieces[0].TrackID

	// A gray piece where the red one was predicted: not the same piece.
	obs = Observe(z, []Box{at(232, 20)}, 4, now.Add(100*time.Millisecond))
	tr.Update(z, &obs, [][3]float64{{0.5, 0.5, 0.5}})
	assert.NotEqual(t, first, obs.Pieces[0].TrackID)
}
