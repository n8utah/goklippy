package routes

import (
	"errors"
	"net/http"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var sorterSharing *sorter.Sharing

// InitSharing wires sending to the sort viewer site into the routes.
func InitSharing(s *sorter.Sharing) {
	sorterSharing = s
}

// GetSorterSharing GET /sorter/sharing: the site, whether a key is set (never the key)
// and how sending is going.
func GetSorterSharing(c *gin.Context) {
	status, err := sorterSharing.Status(c.Request.Context())
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

type sharingRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// PutSorterSharing PUT /sorter/sharing {"url": "https://lego.n8p.xyz", "api_key": "sv_..."}:
// an empty api_key keeps the saved one.
func PutSorterSharing(c *gin.Context) {
	var req sharingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	ctx := c.Request.Context()
	if err := sorterSharing.SaveSettings(ctx, req.URL, req.APIKey); err != nil {
		sorterError(c, err)
		return
	}
	GetSorterSharing(c)
}

// TestSorterSharing POST /sorter/sharing/test: checks the site answers and takes the key.
func TestSorterSharing(c *gin.Context) {
	if err := sorterSharing.Test(c.Request.Context()); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sorter.ErrSharingNotSetUp) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
