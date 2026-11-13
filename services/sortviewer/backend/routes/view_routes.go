package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "id must be a number"})
		return 0, false
	}
	return id, true
}

// listCollections GET /api/collections: the ones you may see.
func (a *API) listCollections(c *gin.Context) {
	sess, _ := session(c)
	cols, err := a.Svc.Collections(c.Request.Context(), sess)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"collections": cols})
}

// getCollection GET /api/collections/:id
func (a *API) getCollection(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	col, err := a.Svc.Collection(c.Request.Context(), sess, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, col)
}

// getPlan GET /api/collections/:id/plan: its bins and what each has had.
func (a *API) getPlan(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	plan, err := a.Svc.Plan(c.Request.Context(), sess, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}

// listPieces GET /api/collections/:id/pieces?unknown=1&q=&limit=&offset=
func (a *API) listPieces(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "60"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	sess, _ := session(c)
	pieces, total, err := a.Svc.Pieces(c.Request.Context(), sess, id, logic.PieceFilter{
		Unknown: c.Query("unknown") == "1", Search: c.Query("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pieces": pieces, "total": total})
}

// listParts GET /api/collections/:id/parts: counts by part and colour.
func (a *API) listParts(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	parts, err := a.Svc.Parts(c.Request.Context(), sess, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"parts": parts})
}

// listRuns GET /api/collections/:id/runs
func (a *API) listRuns(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	runs, err := a.Svc.Runs(c.Request.Context(), sess, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// eventsKeepAlive is how often a quiet event stream says it's still there, so proxies
// don't close it.
const eventsKeepAlive = 25 * time.Second

// events GET /api/collections/:id/events: server-sent events as the sorter pushes.
func (a *API) events(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	if err := a.Svc.Access(c.Request.Context(), sess, id); err != nil {
		fail(c, err)
		return
	}
	ch, stop := a.Svc.Events.Subscribe(id)
	defer stop()
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": hello\n\n")
	w.Flush()
	tick := time.NewTicker(eventsKeepAlive)
	defer tick.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": still here\n\n")
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		w.Flush()
	}
}

// photo GET /api/photos/:piece: a piece's photo.
func (a *API) photo(c *gin.Context) {
	sess, _ := session(c)
	path, err := a.Svc.Photo(c.Request.Context(), sess, c.Param("piece"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.File(path)
}
