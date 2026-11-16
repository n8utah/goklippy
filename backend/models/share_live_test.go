package models

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/sorter"
)

// fakeLiveSite answers the sorter's live reports with however many are watching, and
// keeps what it's sent.
type fakeLiveSite struct {
	mu        sync.Mutex
	watchers  int
	reported  []int64 // the collection in each report
	frames    map[string]int
	state     sorter.LiveState
	stateSeen bool
}

func (f *fakeLiveSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == "POST" && r.URL.Path == "/api/push/live":
		var in struct {
			CollectionID int64 `json:"collection_id"`
			Watchers     int   `json:"watchers"`
		}
		_ = json.Unmarshal(body, &in)
		f.reported = append(f.reported, in.CollectionID)
		n := f.watchers
		if in.Watchers == n {
			f.mu.Unlock()
			time.Sleep(30 * time.Millisecond) // the site holds a report while nothing changes
			f.mu.Lock()
			n = f.watchers
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"watchers": n})
	case strings.Contains(r.URL.Path, "/frames/"):
		f.frames[r.URL.Path]++
		w.WriteHeader(http.StatusNoContent)
	case strings.HasSuffix(r.URL.Path, "/state"):
		_ = json.Unmarshal(body, &f.state)
		f.stateSeen = true
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeLiveSite) lastReported() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reported) == 0 {
		return -1
	}
	return f.reported[len(f.reported)-1]
}

func (f *fakeLiveSite) sentFrames() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.frames {
		n += c
	}
	return n
}

// The site is told which shared collection is sorting, and only sent the cameras and
// the state while someone is watching it.
func TestSharingLiveOnlyWhileWatched(t *testing.T) {
	r := newShareRig(t)
	sam, alex := newCollection(t, r.records, "Sam"), newCollection(t, r.records, "Alex")
	require.NoError(t, r.share.SetShared(r.ctx, sam, true))
	run, err := r.records.StartRun(r.ctx, sam, nil, "", time.Now().Add(-2*time.Minute))
	require.NoError(t, err)
	r.piece(run, "3001")

	site := &fakeLiveSite{frames: map[string]int{}}
	srv := httptest.NewServer(site)
	t.Cleanup(srv.Close)
	require.NoError(t, r.share.SaveSettings(r.ctx, srv.URL, ""))

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1280, 720)), nil))
	var mu sync.Mutex
	sorting := sorter.CollectionRef{ID: sam, Name: "Sam"}
	runID := run
	snapshot := func() sorter.Snapshot {
		mu.Lock()
		defer mu.Unlock()
		return sorter.Snapshot{State: sorter.StateRunning, RunID: runID, Collection: &sorting,
			Doing: map[sorter.Channel]string{sorter.ChannelC2: "carrying a piece to the exit"}}
	}
	frame := func(sorter.Channel) ([]byte, time.Time, bool) { return buf.Bytes(), time.Now(), true }
	r.share.AttachLive(sorter.LiveSource{Snapshot: snapshot, Frame: frame})
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	go r.share.RunLive(ctx)

	// Nobody watching: it says what's sorting, and sends nothing else.
	require.Eventually(t, func() bool { return site.lastReported() == sam }, 3*time.Second, 10*time.Millisecond)
	time.Sleep(300 * time.Millisecond)
	assert.Zero(t, site.sentFrames())

	// Someone watching: each camera, and the state with the run's numbers.
	site.mu.Lock()
	site.watchers = 1
	site.mu.Unlock()
	require.Eventually(t, func() bool {
		site.mu.Lock()
		defer site.mu.Unlock()
		return site.stateSeen && len(site.frames) == 3
	}, 5*time.Second, 20*time.Millisecond)
	site.mu.Lock()
	st := site.state
	site.mu.Unlock()
	assert.Equal(t, sorter.StateRunning, st.State)
	assert.Equal(t, "carrying a piece to the exit", st.Doing[sorter.ChannelC2])
	require.NotNil(t, st.Run)
	assert.Equal(t, 1, st.Run.Sorted)
	assert.InDelta(t, 0.5, st.Run.PerMinute, 0.05, "one sorted in two minutes")
	n, watching := r.share.LiveWatchers()
	assert.Equal(t, 1, n)
	assert.Equal(t, sam, watching)

	// Sorting another collection, not a shared one: nothing is said to be sorting, and
	// nothing more is sent.
	mu.Lock()
	sorting = sorter.CollectionRef{ID: alex, Name: "Alex"}
	mu.Unlock()
	require.Eventually(t, func() bool { return site.lastReported() == 0 }, 5*time.Second, 20*time.Millisecond)
	time.Sleep(400 * time.Millisecond)
	before := site.sentFrames()
	time.Sleep(600 * time.Millisecond)
	assert.Equal(t, before, site.sentFrames())
}
