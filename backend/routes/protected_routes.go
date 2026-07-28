package routes

import (
	"net/http"

	"goklippy/backend/logic"

	"github.com/gin-gonic/gin"
)

func Me(c *gin.Context) {
	claimsValue, ok := c.Get("claims")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	claims, ok := claimsValue.(*logic.AccessClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": claims.UserID,
		"email":   claims.Email,
		"role":    claims.Role,
	})
}
