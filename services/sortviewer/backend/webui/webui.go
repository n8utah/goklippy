// Package webui serves the built frontend, embedded in the binary (mage build copies
// frontend/dist here first).
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the pages: a file when one matches, else index.html (the React app
// routes the rest). Unknown /api paths get a JSON 404.
func Handler() gin.HandlerFunc {
	files, _ := fs.Sub(dist, "dist")
	server := http.FileServer(http.FS(files))
	index, indexErr := fs.ReadFile(files, "index.html")
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/auth/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "no such endpoint"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && name != "index.html" {
			if f, err := files.Open(name); err == nil {
				f.Close()
				if strings.HasPrefix(name, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				server.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		if indexErr != nil {
			c.String(http.StatusOK, "The web pages aren't built into this server. Run: mage build")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
