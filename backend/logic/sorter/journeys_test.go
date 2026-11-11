package sorter

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPiece is a checkered square (crisp, so sharp) of size px at angle on the sim ring.
type testPiece struct {
	id    int
	angle float64
	size  int
	flat  bool // a smooth blob instead: blurry
	at    *image.Point
}

func pieceBox(p testPiece) vision.Box {
	z := SimZones()
	cx, cy := z.Ring.Center.X+150*math.Cos(p.angle*math.Pi/180), z.Ring.Center.Y+150*math.Sin(p.angle*math.Pi/180)
	if p.at != nil {
		cx, cy = float64(p.at.X), float64(p.at.Y)
	}
	h := p.size / 2
	return vision.Box{X1: int(cx) - h, Y1: int(cy) - h, X2: int(cx) + h, Y2: int(cy) + h}
}

// frameOf draws the pieces and returns the frame and the observation perception
// would give, with the track ids already assigned.
func frameOf(at time.Time, pieces ...testPiece) (image.Image, vision.Observation) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 128, 128, 128, 255
	}
	z := SimZones()
	obs := vision.Observation{At: at, Valid: true, Pieces: []vision.PieceObs{}}
	for _, p := range pieces {
		b := pieceBox(p)
		for y := max(0, b.Y1); y < min(480, b.Y2); y++ {
			for x := max(0, b.X1); x < min(640, b.X2); x++ {
				if p.flat {
					// A soft smudge barely off the background: no detail, no edges.
					img.SetRGBA(x, y, color.RGBA{124, 124, 124, 255})
					continue
				}
				v := uint8(30)
				if ((x-b.X1)/4+(y-b.Y1)/4)%2 == 0 {
					v = 230
				}
				img.SetRGBA(x, y, color.RGBA{v, v / 2, 20, 255})
			}
		}
		angle := z.AngleOf(b.Center())
		obs.Pieces = append(obs.Pieces, vision.PieceObs{Box: b, Angle: angle, Zone: z.ZoneAt(angle), TrackID: p.id})
	}
	return img, obs
}

func newRecorder(t *testing.T, pics PicturesSettings) (*JourneyRecorder, string) {
	t.Helper()
	dir := t.TempDir()
	settings := func() MachineSettings {
		s := DefaultMachineSettings()
		s.Pictures = pics
		return s
	}
	return NewJourneyRecorder(dir, settings), dir
}

func journey(t *testing.T, r *JourneyRecorder, id string) Journey {
	t.Helper()
	raw, err := r.Get(id)
	require.NoError(t, err)
	var j Journey
	require.NoError(t, json.Unmarshal(raw, &j))
	return j
}

func TestJourneyKeepsSpreadOutPicturesAndFinishes(t *testing.T) {
	r, dir := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	// One piece rides from 200° to 345°, 5° a frame.
	for i := 0; i <= 29; i++ {
		img, obs := frameOf(t0.Add(time.Duration(i)*100*time.Millisecond), testPiece{id: 7, angle: 200 + 5*float64(i), size: 26})
		r.Observe(ChannelC3, img, SimZones(), obs)
	}
	list := r.List(10)
	require.Len(t, list, 1)
	assert.False(t, list[0].Done)
	j := journey(t, r, list[0].ID)
	frames := j.Tracks[0].Frames
	require.Len(t, frames, 30)
	for i := 0; i < 3; i++ {
		assert.Equal(t, ReasonSettling, frames[i].Reason, "frame %d", i)
	}
	// 145° of travel in 15° stretches: at most 10 kept, each its stretch's sharpest.
	assert.InDelta(t, 9, j.Kept, 1)
	assert.Positive(t, j.Reasons[ReasonSharper])
	assert.NotEmpty(t, j.Thumb)

	// Unseen past the timeout: finished, saved with its pictures.
	img, obs := frameOf(t0.Add(7 * time.Second))
	r.Observe(ChannelC3, img, SimZones(), obs)
	list = r.List(10)
	require.Len(t, list, 1)
	assert.True(t, list[0].Done)
	assert.Equal(t, "left by the exit", list[0].EndReason)
	j = journey(t, r, list[0].ID)
	require.Len(t, j.Tracks[0].Moments, 2)
	assert.Equal(t, "Landed", j.Tracks[0].Moments[0].Label)
	assert.Equal(t, "Last seen", j.Tracks[0].Moments[1].Label)
	for _, name := range []string{"journey.json", j.Thumb, frames[0].Thumb, j.Tracks[0].Moments[1].Image} {
		_, err := os.Stat(filepath.Join(dir, j.ID, name))
		assert.NoError(t, err, name)
	}

	// A new recorder loads it back.
	r2 := NewJourneyRecorder(dir, r.settings)
	require.Len(t, r2.List(10), 1)
	assert.Equal(t, j.ID, r2.List(10)[0].ID)
}

func TestJourneyRejectsBadPictures(t *testing.T) {
	pics := DefaultPictures()
	pics.SettleFrames = 1
	r, _ := newRecorder(t, pics)
	t0 := time.Now()
	step := func(i int, pieces ...testPiece) {
		img, obs := frameOf(t0.Add(time.Duration(i)*100*time.Millisecond), pieces...)
		r.Observe(ChannelC3, img, SimZones(), obs)
	}
	step(0, testPiece{id: 1, angle: 220, size: 26})                                         // settling
	step(1, testPiece{id: 1, angle: 222, size: 26})                                         // kept
	step(2, testPiece{id: 1, angle: 224, size: 26}, testPiece{id: 2, angle: 232, size: 26}) // not alone
	step(3, testPiece{id: 1, angle: 250, size: 26})                                         // moving: 26° in a frame
	step(4, testPiece{id: 1, angle: 252, size: 44})                                         // size jump
	step(5, testPiece{id: 1, angle: 254, size: 26, flat: true})                             // blurry
	step(6, testPiece{id: 1, angle: 256, size: 26, at: &image.Point{X: 630, Y: 300}})       // cut off by the frame's edge
	step(7, testPiece{id: 1, angle: 258, size: 60, at: &image.Point{X: 320 + 190, Y: 240}}) // reaches past the ring
	var j Journey
	for _, s := range r.List(10) {
		if js := journey(t, r, s.ID); js.Tracks[0].TrackID == 1 {
			j = js
		}
	}
	reasons := []string{}
	for _, f := range j.Tracks[0].Frames {
		if f.Kept {
			reasons = append(reasons, "kept")
		} else {
			reasons = append(reasons, f.Reason)
		}
	}
	assert.Equal(t, []string{ReasonSettling, "kept", ReasonNotAlone, ReasonMoving, ReasonSizeJump, ReasonBlurry, ReasonCutOff, ReasonCutOff}, reasons)
}

func TestJourneyKeepsAtMostMaxKept(t *testing.T) {
	pics := DefaultPictures()
	pics.MaxKept, pics.KeepEveryDeg, pics.SettleFrames = 3, 4, 1
	r, _ := newRecorder(t, pics)
	t0 := time.Now()
	for i := 0; i < 20; i++ {
		img, obs := frameOf(t0.Add(time.Duration(i)*100*time.Millisecond), testPiece{id: 3, angle: 210 + 5*float64(i), size: 26})
		r.Observe(ChannelC3, img, SimZones(), obs)
	}
	j := journey(t, r, r.List(1)[0].ID)
	assert.Equal(t, 3, j.Kept)
	assert.Positive(t, j.Reasons[ReasonOverMax])
}

func TestJourneyFilesStayInTheirFolder(t *testing.T) {
	r, _ := newRecorder(t, DefaultPictures())
	_, err := r.File("../etc", "t-0001.jpg")
	assert.ErrorIs(t, err, ErrNoJourney)
	_, err = r.File("20261111-c3-1", "../journey.json")
	assert.ErrorIs(t, err, ErrNoJourney)
	_, err = r.Get("..")
	assert.ErrorIs(t, err, ErrNoJourney)
	path, err := r.File("20261111-c3-1", "k-0003.jpg")
	require.NoError(t, err)
	assert.Equal(t, "k-0003.jpg", filepath.Base(path))
}
