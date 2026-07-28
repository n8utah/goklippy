package main

import (
	"goklippy/backend/config"
	"goklippy/backend/routes"

	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine, cfg config.Config) {
	api := r.Group("/api/v1")

	api.GET("/health", routes.Health)
	api.POST("/auth/login", routes.LoginHandler())
	api.POST("/auth/refresh", routes.RefreshHandler())
	api.POST("/auth/logout", routes.LogoutHandler())
	api.GET("/auth/me", routes.RequireJWT(), routes.Me)

	registerFrontendHosting(r, cfg.Frontend)
}
