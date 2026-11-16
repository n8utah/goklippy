package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/services/sortviewer/backend/logic"
)

// watch opens a page's live stream and returns its events as they come.
func (c *client) watch(path string) (int, <-chan logic.LiveEvent, func()) {
	c.site.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", c.site.srv.URL+path, nil)
	require.NoError(c.site.t, err)
	resp, err := c.http.Do(req)
	require.NoError(c.site.t, err)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return resp.StatusCode, nil, func() {}
	}
	events := make(chan logic.LiveEvent, 64)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		lines := bufio.NewScanner(resp.Body)
		for lines.Scan() {
			if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
				var e logic.LiveEvent
				if json.Unmarshal([]byte(data), &e) == nil {
					events <- e
				}
			}
		}
	}()
	return resp.StatusCode, events, cancel
}

func next(t *testing.T, events <-chan logic.LiveEvent) logic.LiveEvent {
	t.Helper()
	select {
	case e, ok := <-events:
		require.True(t, ok, "the stream ended")
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("no event")
	}
	return logic.LiveEvent{}
}

// A collection can be watched only while the sorter says it's sorting it, and only by
// those who can see it; the sorter hears when someone starts watching, and the pages
// when it stops.
func TestWatchingASortLive(t *testing.T) {
	s := newSite(t)
	s.pushSort()
	admin := s.browser()
	code, _ := admin.do("GET", "/auth/login", nil)
	require.Equal(t, http.StatusFound, code)
	code, _ = admin.do("POST", "/api/admin/viewers", map[string]any{"email": "sam@example.com", "password": "lego-lego", "collections": []int{7}})
	require.Equal(t, 201, code)
	code, _ = admin.do("POST", "/api/admin/viewers", map[string]any{"email": "alex@example.com", "password": "bricks-bricks"})
	require.Equal(t, 201, code)
	sam, alex := s.browser(), s.browser()
	code, _ = sam.do("POST", "/api/auth/login", map[string]any{"email": "sam@example.com", "password": "lego-lego"})
	require.Equal(t, 200, code)
	code, _ = alex.do("POST", "/api/auth/login", map[string]any{"email": "alex@example.com", "password": "bricks-bricks"})
	require.Equal(t, 200, code)
	sorter := s.sorter(s.key)

	// Not sorting: nothing to watch.
	_, body := sam.do("GET", "/api/collections/7", nil)
	assert.Equal(t, false, body["live"])
	code, _, _ = sam.watch("/api/collections/7/live")
	assert.Equal(t, http.StatusConflict, code)

	code, body = sorter.do("POST", "/api/push/live", map[string]any{"collection_id": 7, "watchers": -1})
	require.Equal(t, 200, code)
	assert.EqualValues(t, 0, body["watchers"])
	_, body = sam.do("GET", "/api/collections/7", nil)
	assert.Equal(t, true, body["live"])
	code, _, _ = alex.watch("/api/collections/7/live")
	assert.Equal(t, http.StatusNotFound, code, "not Alex's to see")
	code, _ = s.sorter("sv_wrong").do("POST", "/api/push/live", map[string]any{"collection_id": 7})
	assert.Equal(t, http.StatusUnauthorized, code)

	// The sorter waits to hear of a watcher; Sam opening the page tells it.
	heard := make(chan float64, 1)
	go func() {
		_, body := sorter.do("POST", "/api/push/live", map[string]any{"collection_id": 7, "watchers": 0})
		heard <- body["watchers"].(float64)
	}()
	time.Sleep(50 * time.Millisecond)
	code, events, stop := sam.watch("/api/collections/7/live")
	require.Equal(t, http.StatusOK, code)
	defer stop()
	select {
	case n := <-heard:
		assert.EqualValues(t, 1, n)
	case <-time.After(3 * time.Second):
		t.Fatal("the sorter wasn't told")
	}

	// What it sends goes straight to the page.
	code, _ = sorter.do("PUT", "/api/push/live/7/state", map[string]any{"state": "running", "run": map[string]any{"sorted": 12}})
	require.Equal(t, 204, code)
	e := next(t, events)
	assert.Equal(t, "state", e.Type)
	assert.JSONEq(t, `{"state":"running","run":{"sorted":12}}`, string(e.State))
	frame := jpegBytes(t, 64, 48)
	code, _ = sorter.do("PUT", "/api/push/live/7/frames/c2", frame)
	require.Equal(t, 204, code)
	e = next(t, events)
	assert.Equal(t, "frame", e.Type)
	assert.Equal(t, "c2", e.Channel)
	resp, err := sam.http.Get(s.srv.URL + "/api/collections/7/live/frames/c2")
	require.NoError(t, err)
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, frame, got)
	code, _ = sorter.do("PUT", "/api/push/live/7/frames/c9", frame)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = sorter.do("PUT", "/api/push/live/8/state", map[string]any{})
	assert.Equal(t, http.StatusConflict, code, "not the collection sorting")

	// Sorting stops: the page is told, and there's nothing more to see.
	code, _ = sorter.do("POST", "/api/push/live", map[string]any{"collection_id": 0, "watchers": 1})
	require.Equal(t, 200, code)
	assert.Equal(t, "ended", next(t, events).Type)
	code, _ = sam.do("GET", "/api/collections/7/live/frames/c2", nil)
	assert.Equal(t, http.StatusConflict, code)
	_, body = sam.do("GET", "/api/collections/7", nil)
	assert.Equal(t, false, body["live"])
}
