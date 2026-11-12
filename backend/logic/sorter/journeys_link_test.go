package sorter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rideToExit feeds C3 frames of the pieces riding into the exit zone (330°–350°), one
// frame per 100 ms from t0, and returns the time after the last.
func rideToExit(r *JourneyRecorder, t0 time.Time, ids ...int) time.Time {
	at := t0
	for i := 0; i <= 10; i++ {
		var ps []testPiece
		for k, id := range ids {
			ps = append(ps, testPiece{id: id, angle: 290 + 5*float64(i) - 12*float64(k), size: 26})
		}
		img, obs := frameOf(at, ps...)
		r.Observe(ChannelC3, img, SimZones(), obs)
		at = at.Add(100 * time.Millisecond)
	}
	return at
}

// emptyFrame feeds a frame of a channel with nothing on it.
func emptyFrame(r *JourneyRecorder, ch Channel, at time.Time) {
	img, obs := frameOf(at)
	r.Observe(ch, img, SimZones(), obs)
}

// land feeds a C4 frame with the pieces in its drop zone (200°–260°).
func land(r *JourneyRecorder, at time.Time, ids ...int) {
	var ps []testPiece
	for k, id := range ids {
		ps = append(ps, testPiece{id: id, angle: 215 + 30*float64(k), size: 26})
	}
	img, obs := frameOf(at, ps...)
	r.Observe(ChannelC4, img, SimZones(), obs)
}

func TestJourneyCarriesOnWhenOnePieceFalls(t *testing.T) {
	r, dir := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	land(r, t0) // C4's camera is running
	at := rideToExit(r, t0, 5)
	// It's gone from C3, and 300 ms later one piece lands on C4.
	emptyFrame(r, ChannelC3, at)
	land(r, at.Add(300*time.Millisecond), 9)

	list := r.List(10)
	require.Len(t, list, 1, "one journey, not two")
	j := journey(t, r, list[0].ID)
	require.Len(t, j.Tracks, 2)
	assert.Equal(t, ChannelC3, j.Tracks[0].Channel)
	assert.Equal(t, "fell to C4", j.Tracks[0].EndReason)
	assert.Equal(t, ChannelC4, j.Tracks[1].Channel)
	require.Len(t, j.Handoffs, 1)
	assert.Equal(t, HandoffLinked, j.Handoffs[0].Result)
	assert.Equal(t, 1, j.Handoffs[0].Candidates)
	assert.InDelta(t, 400, j.Handoffs[0].GapMs, 1)

	// Both tracks' pictures sit in the journey's folder under their own names.
	c3, c4 := j.Tracks[0], j.Tracks[1]
	assert.NotEqual(t, c3.Frames[0].Thumb, c4.Frames[0].Thumb)
	for _, name := range []string{c3.Frames[0].Thumb, c4.Frames[0].Thumb, c3.Moments[len(c3.Moments)-1].Image, c4.Moments[0].Image} {
		_, err := os.Stat(filepath.Join(dir, j.ID, name))
		assert.NoError(t, err, name)
	}

	// The platter records the piece as it lands: the journey becomes that piece.
	assert.Equal(t, j.ID, r.NotePiece("piece-1", at.Add(300*time.Millisecond)))
	raw, err := r.ForPiece("piece-1")
	require.NoError(t, err)
	assert.Contains(t, string(raw), j.ID)
	_, err = r.ForPiece("someone-else")
	assert.ErrorIs(t, err, ErrNoJourney)
}

func TestJourneyStartsAfreshWhenUnsure(t *testing.T) {
	t.Run("two pieces fell, one landed", func(t *testing.T) {
		r, _ := newRecorder(t, DefaultPictures())
		t0 := time.Now()
		land(r, t0)
		at := rideToExit(r, t0, 5, 6)
		emptyFrame(r, ChannelC3, at)
		land(r, at.Add(200*time.Millisecond), 9)
		c4 := newest(t, r)
		require.Len(t, c4.Tracks, 1)
		require.Len(t, c4.Handoffs, 1)
		assert.Equal(t, HandoffSeveral, c4.Handoffs[0].Result)
		assert.Equal(t, 2, c4.Handoffs[0].Candidates)
	})

	t.Run("one fell, two landed together", func(t *testing.T) {
		r, _ := newRecorder(t, DefaultPictures())
		t0 := time.Now()
		land(r, t0)
		at := rideToExit(r, t0, 5)
		emptyFrame(r, ChannelC3, at)
		land(r, at.Add(200*time.Millisecond), 9, 10)
		for _, s := range r.List(10) {
			j := journey(t, r, s.ID)
			assert.Len(t, j.Tracks, 1, "nothing linked")
		}
	})

	t.Run("landed too late", func(t *testing.T) {
		r, _ := newRecorder(t, DefaultPictures())
		t0 := time.Now()
		land(r, t0)
		at := rideToExit(r, t0, 5)
		emptyFrame(r, ChannelC3, at)
		emptyFrame(r, ChannelC3, at.Add(time.Second))
		land(r, at.Add(2*time.Second), 9)
		emptyFrame(r, ChannelC3, at.Add(4*time.Second)) // the window has passed
		c4 := newest(t, r)
		require.Len(t, c4.Handoffs, 1)
		assert.Equal(t, HandoffNone, c4.Handoffs[0].Result)
	})

	t.Run("still on the channel above", func(t *testing.T) {
		r, _ := newRecorder(t, DefaultPictures())
		t0 := time.Now()
		land(r, t0)
		at := rideToExit(r, t0, 5) // at the exit, and it stays there
		land(r, at, 9)
		assert.Empty(t, newest(t, r).Handoffs, "waiting to see whether it fell")
		for k := 1; k <= 20; k++ {
			img, obs := frameOf(at.Add(time.Duration(k)*100*time.Millisecond), testPiece{id: 5, angle: 340, size: 26})
			r.Observe(ChannelC3, img, SimZones(), obs)
		}
		c4 := newest(t, r)
		require.Len(t, c4.Handoffs, 1)
		assert.Equal(t, HandoffNone, c4.Handoffs[0].Result)
		assert.Len(t, c4.Tracks, 1)
	})
}

// The camera below can see the piece land before the camera above sends the frame
// showing it gone: the landing waits, then links, and its pictures join the journey.
func TestJourneyLinksWhenTheCameraAboveIsBehind(t *testing.T) {
	r, dir := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	land(r, t0)
	at := rideToExit(r, t0, 5)
	// C4 first, a few frames, while C3 hasn't sent a frame since the piece left.
	for k := 0; k < 3; k++ {
		land(r, at.Add(time.Duration(k)*30*time.Millisecond), 9)
	}
	require.Len(t, r.List(10), 2, "two journeys while it waits")
	emptyFrame(r, ChannelC3, at.Add(100*time.Millisecond))

	list := r.List(10)
	require.Len(t, list, 1, "one journey once C3 shows the piece gone")
	j := journey(t, r, list[0].ID)
	require.Len(t, j.Tracks, 2)
	assert.Equal(t, HandoffLinked, j.Handoffs[0].Result)
	assert.Equal(t, 3, len(j.Tracks[1].Frames))
	for _, f := range j.Tracks[1].Frames {
		_, err := os.Stat(filepath.Join(dir, j.ID, f.Thumb))
		assert.NoError(t, err, "%s moved into the journey's folder", f.Thumb)
	}
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1, "the waiting journey's folder is gone")
}

func TestJourneyWithNoCameraOnC4BecomesThePiece(t *testing.T) {
	r, _ := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	at := rideToExit(r, t0, 5)
	emptyFrame(r, ChannelC3, at)
	id := r.NotePiece("piece-2", at.Add(300*time.Millisecond))
	require.NotEmpty(t, id)
	assert.Equal(t, "piece-2", r.List(1)[0].PieceID)
	// Taken: a second piece can't claim the same journey.
	assert.Empty(t, r.NotePiece("piece-3", at.Add(400*time.Millisecond)))
}

func TestJourneyWithNoCameraAboveHasNoHandoff(t *testing.T) {
	r, _ := newRecorder(t, DefaultPictures())
	land(r, time.Now(), 9) // C3 has never sent a frame
	assert.Empty(t, newest(t, r).Handoffs)
}

// newest is the most recently started journey.
func newest(t *testing.T, r *JourneyRecorder) Journey {
	t.Helper()
	list := r.List(1)
	require.NotEmpty(t, list)
	return journey(t, r, list[0].ID)
}
