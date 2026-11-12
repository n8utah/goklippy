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
	assert.Equal(t, []vision.Box{stuck}, ignored)

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
