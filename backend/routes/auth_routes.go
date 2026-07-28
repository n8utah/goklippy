package routes

import (
	"errors"
	"net/http"

	"goklippy/backend/logic"
	"goklippy/backend/models"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type tokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func LoginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		service := getAuthService()

		var request loginRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		tokens, err := service.Login(c.Request.Context(), request.Email, request.Password)
		if err != nil {
			handleAuthError(c, err)
			return
		}

		c.JSON(http.StatusOK, tokens)
	}
}

func RefreshHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		service := getAuthService()

		var request tokenRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		tokens, err := service.Refresh(c.Request.Context(), request.RefreshToken)
		if err != nil {
			handleAuthError(c, err)
			return
		}

		c.JSON(http.StatusOK, tokens)
	}
}

func LogoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		service := getAuthService()

		var request tokenRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if err := service.Logout(c.Request.Context(), request.RefreshToken); err != nil {
			handleAuthError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
	}
}

func handleAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, logic.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, logic.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
	case errors.Is(err, models.ErrDBNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication failed"})
	}
}
