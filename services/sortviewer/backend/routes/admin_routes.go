package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

// listViewers GET /api/admin/viewers
func (a *API) listViewers(c *gin.Context) {
	viewers, err := a.Svc.Viewers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"viewers": viewers})
}

// createViewer POST /api/admin/viewers {"email", "name", "password", "collections": [ids]}
func (a *API) createViewer(c *gin.Context) {
	var in logic.ViewerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	id, err := a.Svc.CreateViewer(c.Request.Context(), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// updateViewer PUT /api/admin/viewers/:id {"name", "password" ("" keeps it), "collections"}
func (a *API) updateViewer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in logic.ViewerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := a.Svc.UpdateViewer(c.Request.Context(), id, in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteViewer DELETE /api/admin/viewers/:id
func (a *API) deleteViewer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.Svc.DeleteViewer(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// listKeys GET /api/admin/keys
func (a *API) listKeys(c *gin.Context) {
	keys, err := a.Svc.APIKeys(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// createKey POST /api/admin/keys {"name"}: the new key, shown this once.
func (a *API) createKey(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is needed"})
		return
	}
	key, err := a.Svc.CreateAPIKey(c.Request.Context(), req.Name)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key})
}

// revokeKey DELETE /api/admin/keys/:id
func (a *API) revokeKey(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.Svc.RevokeAPIKey(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// clearPhotos POST /api/admin/collections/:id/clear-photos: removes its photos.
func (a *API) clearPhotos(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	n, err := a.Svc.ClearPhotos(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": n})
}

// clearAllPhotos POST /api/admin/photos/clear: removes every collection's photos.
func (a *API) clearAllPhotos(c *gin.Context) {
	n, err := a.Svc.ClearPhotos(c.Request.Context(), 0)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": n})
}

// deleteCollection DELETE /api/admin/collections/:id: it, its pieces and photos.
func (a *API) deleteCollection(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := a.Svc.DeleteCollection(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
