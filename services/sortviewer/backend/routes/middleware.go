// Package routes maps HTTP requests to the sort viewer's logic.
package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/config"
	"goklippy/services/sortviewer/backend/logic"
)

const sessionCookie = "sv_session"

// API holds what the handlers need.
type API struct {
	Svc    *logic.Service
	Admin  *logic.AdminSignIn
	Cfg    config.Config
	Commit string
}

// RequestLog logs each request with its status and time.
func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.Request.URL.Path == "/api/health" {
			return
		}
		slog.Info("request", "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "ms", time.Since(start).Milliseconds())
	}
}

// loadSession puts the cookie's session, if any, on the request.
func (a *API) loadSession(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil {
		if sess, err := a.Svc.Session(c.Request.Context(), token); err == nil {
			c.Set("session", sess)
		}
	}
	c.Next()
}

func session(c *gin.Context) (logic.Session, bool) {
	v, ok := c.Get("session")
	if !ok {
		return logic.Session{}, false
	}
	return v.(logic.Session), true
}

func (a *API) requireSession(c *gin.Context) {
	if _, ok := session(c); !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign in first"})
		return
	}
	c.Next()
}

func (a *API) requireAdmin(c *gin.Context) {
	if sess, ok := session(c); !ok || !sess.Admin() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admins only"})
		return
	}
	c.Next()
}

// sameOrigin refuses cookie-authenticated changes sent from another site.
func (a *API) sameOrigin(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		if origin := c.GetHeader("Origin"); origin != "" && !a.originAllowed(origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-site request refused"})
			return
		}
	}
	c.Next()
}

func (a *API) originAllowed(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	app, _ := url.Parse(a.Cfg.AppURL)
	if app != nil && strings.EqualFold(u.Host, app.Host) && u.Scheme == app.Scheme {
		return true
	}
	// The Vite dev server proxies to a local server.
	host := u.Hostname()
	return !a.Cfg.Secure() && (host == "localhost" || host == "127.0.0.1")
}

// requireAPIKey lets the sorter in with "Authorization: Bearer sv_...".
func (a *API) requireAPIKey(c *gin.Context) {
	key, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if ok {
		valid, err := a.Svc.CheckAPIKey(c.Request.Context(), strings.TrimSpace(key))
		if err != nil {
			fail(c, err)
			return
		}
		if valid {
			c.Next()
			return
		}
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "a valid API key is needed"})
}

func (a *API) setSessionCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: a.Cfg.Secure(), SameSite: http.SameSiteLaxMode,
	})
}

// fail answers an error with the status that fits it.
func fail(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, logic.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, logic.ErrBadInput), errors.Is(err, logic.ErrEmailTaken):
		status = http.StatusBadRequest
	case errors.Is(err, logic.ErrBadLogin), errors.Is(err, logic.ErrLoginExpired):
		status = http.StatusUnauthorized
	case errors.Is(err, logic.ErrNotAdmin):
		status = http.StatusForbidden
	case errors.Is(err, logic.ErrTooManyLogins):
		status = http.StatusTooManyRequests
	case errors.Is(err, logic.ErrSetsBuilding):
		status = http.StatusConflict
	case errors.Is(err, logic.ErrNoOIDC), errors.Is(err, logic.ErrSetsNotReady):
		status = http.StatusServiceUnavailable
	}
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "path", c.FullPath(), "error", err)
		c.AbortWithStatusJSON(status, gin.H{"error": "something went wrong on the server"})
		return
	}
	c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
}
