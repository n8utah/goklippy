package routes

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"goklippy/backend/logic"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

func RequestContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Header(requestIDHeader, requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

func SlogRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		attrs := []any{
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		if claimsRaw, ok := c.Get("claims"); ok {
			if claims, ok := claimsRaw.(*logic.AccessClaims); ok {
				attrs = append(attrs, "user_id", claims.UserID, "user_email", claims.Email, "user_role", claims.Role)
			}
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		if c.Writer.Status() >= http.StatusInternalServerError {
			slog.Error("http request completed", attrs...)
			return
		}
		slog.Info("http request completed", attrs...)
	}
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}
