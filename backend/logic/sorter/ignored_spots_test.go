package sorter

import (
	"testing"
	"time"

	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
)

func TestIgnoredSpotsLeaveOutOnlyTheStuckPiece(t *testing.T) {
	s := spotIgnores{}
	t0 := time.Now()
	stuck := vision.Box{X1: 100, Y1: 100, X2: 140, Y2: 140}
	s.add(ChannelC3, stuck, 0, t0)

	jittered := vision.Box{X1: 98, Y1: 103, X2: 143, Y2: 141} // the same piece, seen a bit off
	beside := vision.Box{X1: 150, Y1: 100, X2: 180, Y2: 140}  // a new piece next to it
	far := vision.Box{X1: 300, Y1: 300, X2: 330, Y2: 330}
	kept, ignored := s.filter(ChannelC3, []vision.Box{jittered, beside, far}, 2, t0.Add(time.Second))
	assert.ElementsMatch(t, []vision.Box{beside, far}, kept, "only the stuck piece is left out")
	assert.Equal(t, []vision.Box{jittered}, ignored, "where it was last seen")

	// A piece landing right on it makes twice the blob: both are seen.
	kept, _ = s.filter(ChannelC3, []vision.Box{stuck, stuck}, 2, t0.Add(time.Second))
	assert.Len(t, kept, 2)
	// Two copies apart: only one of them is the stuck piece.
	kept, _ = s.filter(ChannelC3, []vision.Box{stuck, {X1: 100, Y1: 100, X2: 140, Y2: 140}}, -100, t0.Add(time.Second))
	assert.Len(t, kept, 1)

	kept, _ = s.filter(ChannelC2, []vision.Box{jittered}, 2, t0)
	assert.Len(t, kept, 1, "other channels are untouched")

	// Still there a while later: still ignored.
	_, ignored = s.filter(ChannelC3, []vision.Box{stuck}, 2, t0.Add(9*time.Second))
	assert.Len(t, ignored, 1)
	// Gone for ignoreForgetAfter: watched again.
	_, ignored = s.filter(ChannelC3, nil, 2, t0.Add(9*time.Second+ignoreForgetAfter))
	assert.Empty(t, ignored)
	assert.NotContains(t, s, ChannelC3)
}

func TestIgnoredSpotFollowsANudgedPieceButNotFar(t *testing.T) {
	s := spotIgnores{}
	t0 := time.Now()
	stuck := vision.Box{X1: 100, Y1: 100, X2: 140, Y2: 140}
	s.add(ChannelC3, stuck, 0, t0)
	moved := func(dx int) vision.Box {
		return vision.Box{X1: stuck.X1 + dx, Y1: stuck.Y1, X2: stuck.X2 + dx, Y2: stuck.Y2}
	}

	// Shaken a little at a time: still the stuck piece, still left out.
	for _, dx := range []int{10, 20, 30, 40, 50} {
		kept, _ := s.filter(ChannelC3, []vision.Box{moved(dx)}, 2, t0.Add(time.Second))
		assert.Empty(t, kept, "nudged %dpx: still left out", dx)
	}
	// Further than ignoreDrift from where it stuck: it's riding, so it's seen.
	kept, _ := s.filter(ChannelC3, []vision.Box{moved(70)}, 2, t0.Add(time.Second))
	assert.Len(t, kept, 1)
}

func TestStillThere(t *testing.T) {
	box := vision.Box{X1: 100, Y1: 100, X2: 140, Y2: 140}
	at := func(b vision.Box) vision.Observation {
		return vision.Observation{Valid: true, Pieces: []vision.PieceObs{{Box: b}}}
	}
	assert.True(t, stillThere(at(vision.Box{X1: 104, Y1: 98, X2: 144, Y2: 138}), box, 0+box.Area()))
	assert.False(t, stillThere(at(vision.Box{X1: 160, Y1: 100, X2: 200, Y2: 140}), box, box.Area()), "pushed on")
	assert.False(t, stillThere(vision.Observation{Valid: true}, box, box.Area()), "gone")
}

func TestStuckAgainNearWhereOneWasFreed(t *testing.T) {
	m := &Machine{}
	now := time.Now()
	box := vision.Box{X1: 100, Y1: 100, X2: 140, Y2: 140}
	assert.False(t, m.stuckAgain(ChannelC3, box, now))
	m.noteFreed(ChannelC3, box, now)
	assert.True(t, m.stuckAgain(ChannelC3, vision.Box{X1: 120, Y1: 100, X2: 160, Y2: 140}, now.Add(time.Minute)))
	assert.False(t, m.stuckAgain(ChannelC2, box, now.Add(time.Minute)), "another channel")
	assert.False(t, m.stuckAgain(ChannelC3, vision.Box{X1: 400, Y1: 400, X2: 440, Y2: 440}, now.Add(time.Minute)), "somewhere else")
	assert.False(t, m.stuckAgain(ChannelC3, box, now.Add(stuckAgainWithin+time.Second)), "long ago")
}

func TestPushChannels(t *testing.T) {
	assert.Equal(t, []Channel{ChannelC3, ChannelC4}, pushChannels(ChannelC3))
	assert.Equal(t, []Channel{ChannelC2, ChannelC3, ChannelC4}, pushChannels(ChannelC2))
	assert.Equal(t, ChannelC2, before(ChannelC3))
	assert.Equal(t, ChannelC1, before(ChannelC2))
	assert.Equal(t, "C3 and C4", channelList(pushChannels(ChannelC3)))
	assert.Equal(t, "C2, C3 and C4", channelList(clearChannels))
}
