package routes

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

// The sorter's push API (docs: services/sortviewer/README.md, "Push API"). Every call
// is an upsert keyed by the sorter's ids, so retries and backfills are safe.

// ping GET /api/push/ping: the URL and key work.
func (a *API) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// pushCollection PUT /api/push/collections/:id {"name", "plan"}
func (a *API) pushCollection(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in logic.PushCollection
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := a.Svc.PushCollection(c.Request.Context(), id, in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// pushClearUnknowns POST /api/push/collections/:id/clear-unknowns
func (a *API) pushClearUnknowns(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	n, err := a.Svc.ClearUnknowns(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": n})
}

// pushRun PUT /api/push/runs/:id {"collection_id", "profile_name", "started_at", "ended_at"}
func (a *API) pushRun(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in logic.PushRun
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := a.Svc.PushRun(c.Request.Context(), id, in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// pushPiece PUT /api/push/pieces/:id with the piece record.
func (a *API) pushPiece(c *gin.Context) {
	var in logic.PushPiece
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := a.Svc.PushPiece(c.Request.Context(), c.Param("id"), in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// pushPhoto PUT /api/push/pieces/:id/photo with the JPEG as the body.
func (a *API) pushPhoto(c *gin.Context) {
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, logic.MaxPhotoBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "couldn't read the photo"})
		return
	}
	if len(data) > logic.MaxPhotoBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "the photo is too big"})
		return
	}
	if err := a.Svc.PushPhoto(c.Request.Context(), c.Param("id"), data); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
