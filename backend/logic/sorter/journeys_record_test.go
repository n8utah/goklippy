package sorter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Not sorting, a piece left on a channel starts no journey; one already open finishes
// as it times out, with its journey.json saved.
func TestJourneysOnlyWhileRecording(t *testing.T) {
	r, dir := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	img, obs := frameOf(t0, testPiece{id: 1, angle: 250, size: 26})
	r.Observe(ChannelC2, img, SimZones(), obs)
	require.Len(t, r.List(10), 1, "recording: followed")

	r.SetRecording(false)
	for i := 1; i <= 50; i++ { // still there, and a new one too, for 5 s
		img, obs := frameOf(t0.Add(time.Duration(i)*100*time.Millisecond), testPiece{id: 1, angle: 250, size: 26}, testPiece{id: 2, angle: 300, size: 26})
		r.Observe(ChannelC2, img, SimZones(), obs)
	}
	list := r.List(10)
	require.Len(t, list, 1, "nothing new while not recording")
	assert.True(t, list[0].Done, "the open one finished")
	assert.FileExists(t, filepath.Join(dir, list[0].ID, "journey.json"))
}

// A journey still open when the backend stopped has no journey.json: its folder is
// removed at start-up rather than piling up.
func TestJourneyFoldersWithoutAJourneyAreRemovedAtStart(t *testing.T) {
	r, dir := newRecorder(t, DefaultPictures())
	t0 := time.Now()
	for i := 0; i <= 5; i++ {
		img, obs := frameOf(t0.Add(time.Duration(i)*100*time.Millisecond), testPiece{id: 3, angle: 220 + 5*float64(i), size: 26})
		r.Observe(ChannelC3, img, SimZones(), obs)
	}
	img, obs := frameOf(t0.Add(7 * time.Second))
	r.Observe(ChannelC3, img, SimZones(), obs) // finished: has its journey.json
	orphan := filepath.Join(dir, "20261113-000000-000-c2-9")
	require.NoError(t, os.MkdirAll(orphan, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphan, "k-c20001.jpg"), []byte("x"), 0o644))

	r2 := NewJourneyRecorder(dir, r.settings)
	assert.NoDirExists(t, orphan)
	assert.Len(t, r2.List(10), 1, "the finished one is kept")
}
