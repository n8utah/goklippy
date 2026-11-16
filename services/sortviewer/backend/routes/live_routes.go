package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

// Watching a collection sort (logic/live.go; README, "Live").

const (
	liveStateMax = 256 << 10 // the sorter's state, JSON
	liveFrameMax = 2 << 20   // one camera picture
	// liveCheck: how often a watching page's stream checks its collection is still live
	// (the sorter may go quiet without a word).
	liveCheck = 5 * time.Second
)

// pushLiveReport POST /api/push/live {"collection_id", "watchers"}: the sorter says what
// it's sorting (0: nothing shared) and hears how many are watching, held up to
// logic.LiveWait while that's the number it gave (-1: it doesn't know yet).
func (a *API) pushLiveReport(c *gin.Context) {
	var in logic.LiveReport
	if err := c.ShouldBindJSON(&in); err != nil || in.CollectionID < 0 || in.Watchers < -1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	n := a.Svc.Live.Report(c.Request.Context(), in)
	c.JSON(http.StatusOK, gin.H{"watchers": n})
}

func collectionParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("collection"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "collection must be a number"})
		return 0, false
	}
	return id, true
}

// pushLiveState PUT /api/push/live/:collection/state with the sorter's state (JSON,
// passed on to the pages as it is).
func (a *API) pushLiveState(c *gin.Context) {
	id, ok := collectionParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, liveStateMax+1))
	if err != nil || len(body) > liveStateMax || !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the state should be JSON, at most 256 KB"})
		return
	}
	if err := a.Svc.Live.PutState(id, body); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pushLiveFrame PUT /api/push/live/:collection/frames/:channel with a JPEG.
func (a *API) pushLiveFrame(c *gin.Context) {
	id, ok := collectionParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, liveFrameMax+1))
	if err != nil || len(body) == 0 || len(body) > liveFrameMax {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the picture should be a JPEG of at most 2 MB"})
		return
	}
	if err := a.Svc.Live.PutFrame(id, c.Param("channel"), body); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// watchLive GET /api/collections/:id/live: server-sent events while it sorts: "state",
// "frame" (fetch it from live/frames) and "ended". 409 when it isn't sorting now.
// Watching is what tells the sorter to send anything at all.
func (a *API) watchLive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	events, stop, err := a.Svc.WatchLive(c.Request.Context(), sess, id)
	if err != nil {
		fail(c, err)
		return
	}
	defer stop()
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": watching\n\n")
	w.Flush()
	send := func(e logic.LiveEvent) {
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	check := time.NewTicker(liveCheck)
	defer check.Stop()
	quiet := time.Now()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-check.C:
			if !a.Svc.Live.IsLive(id) {
				send(logic.LiveEvent{Type: "ended"})
				w.Flush()
				return
			}
			if time.Since(quiet) >= eventsKeepAlive {
				fmt.Fprint(w, ": still here\n\n")
				quiet = time.Now()
			}
		case e := <-events:
			send(e)
			quiet = time.Now()
			if e.Type == "ended" {
				w.Flush()
				return
			}
		}
		w.Flush()
	}
}

// liveFrame GET /api/collections/:id/live/frames/:channel: a camera's newest picture.
func (a *API) liveFrame(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	jpeg, err := a.Svc.LiveFrame(c.Request.Context(), sess, id, c.Param("channel"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpeg)
}
