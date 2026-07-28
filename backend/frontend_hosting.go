package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"goklippy/backend/config"

	"github.com/gin-gonic/gin"
)

func registerFrontendHosting(r *gin.Engine, cfg config.FrontendConfig) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "static":
		registerStaticFrontend(r, cfg.DistDir)
	default:
		registerDevProxyFrontend(r, cfg.DevURL)
	}
}

func registerDevProxyFrontend(r *gin.Engine, devURL string) {
	target, err := url.Parse(devURL)
	if err != nil {
		slog.Error("invalid frontend dev url", "dev_url", devURL, "error", err)
		registerFrontendUnavailable(r)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, proxyErr error) {
		slog.Error("frontend dev proxy error", "path", req.URL.Path, "error", proxyErr)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("frontend dev server unavailable"))
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})
}

func registerStaticFrontend(r *gin.Engine, distDir string) {
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		slog.Error("frontend static build not found", "dist_dir", distDir, "error", err)
		registerFrontendUnavailable(r)
		return
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}

		requestPath := path.Clean("/" + c.Request.URL.Path)
		if requestPath == "/" {
			c.File(indexPath)
			return
		}

		filePath := filepath.Join(distDir, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
		if stat, err := os.Stat(filePath); err == nil && !stat.IsDir() {
			c.File(filePath)
			return
		}

		if path.Ext(requestPath) != "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
			return
		}

		c.File(indexPath)
	})
}

func registerFrontendUnavailable(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "frontend is not available"})
	})
}
