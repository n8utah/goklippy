package routes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

// listSets GET /api/collections/:id/sets?q=&min=&any=1&limit=: the sets it covers best.
func (a *API) listSets(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	minParts, _ := strconv.Atoi(c.Query("min"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	sess, _ := session(c)
	ctx := c.Request.Context()
	page, err := a.Svc.BuildableSets(ctx, sess, id, logic.SetFilter{
		Search: c.Query("q"), MinParts: minParts, AnyColour: c.Query("any") == "1", Limit: limit,
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sets": page.Sets, "total": page.Total, "coverage": page.Coverage,
		"built_at": a.Svc.SetsStatus(ctx).BuiltAt,
	})
}

// getSet GET /api/collections/:id/sets/:set: one set's parts against the collection.
func (a *API) getSet(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sess, _ := session(c)
	set, parts, err := a.Svc.SetDetail(c.Request.Context(), sess, id, c.Param("set"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"set": set, "parts": parts})
}

// setsStatus GET /api/admin/sets: the set data's state.
func (a *API) setsStatus(c *gin.Context) {
	c.JSON(http.StatusOK, a.Svc.SetsStatus(c.Request.Context()))
}

// refreshSets POST /api/admin/sets/refresh: rebuild the set data in the background.
func (a *API) refreshSets(c *gin.Context) {
	if err := a.Svc.RefreshSets(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusAccepted, a.Svc.SetsStatus(c.Request.Context()))
}
