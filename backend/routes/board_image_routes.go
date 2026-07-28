package routes

import (
	"net/http"
	"path/filepath"
	"strings"

	"goklippy/backend/profiles"

	"github.com/gin-gonic/gin"
)

// ServeBoardProfileImage GET /board-profiles/:id/image
func ServeBoardProfileImage(c *gin.Context) {
	if boardProfilesDir == "" {
		c.Status(http.StatusNotFound)
		return
	}

	id := c.Param("id")
	p, err := profiles.LoadByID(boardProfilesDir, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if p.Diagram.Image == "" {
		c.Status(http.StatusNotFound)
		return
	}

	path := filepath.Join(boardProfilesDir, filepath.FromSlash(p.Diagram.Image))
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	case ".svg":
		c.Header("Content-Type", "image/svg+xml")
	}
	c.File(path)
}
