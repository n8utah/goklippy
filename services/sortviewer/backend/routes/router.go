package routes

import (
	"github.com/gin-gonic/gin"
)

// Register wires every route. ui serves the web pages for everything else.
func (a *API) Register(r *gin.Engine, ui gin.HandlerFunc) {
	r.Use(a.loadSession, a.sameOrigin)

	r.GET("/api/health", a.health)

	// Sign-in: viewers with email and password, the admin through the Gangway.
	r.POST("/api/auth/login", a.login)
	r.POST("/api/auth/logout", a.logout)
	r.GET("/api/auth/me", a.me)
	r.GET("/auth/login", a.adminLoginStart)
	r.GET("/auth/callback", a.adminLoginCallback)
	r.GET("/auth/logout", a.adminLogout)
	r.POST("/auth/backchannel-logout", a.backchannelLogout)

	// What viewers (and the admin) see.
	view := r.Group("/api", a.requireSession)
	view.GET("/collections", a.listCollections)
	view.GET("/collections/:id", a.getCollection)
	view.GET("/collections/:id/plan", a.getPlan)
	view.GET("/collections/:id/pieces", a.listPieces)
	view.GET("/collections/:id/parts", a.listParts)
	view.GET("/collections/:id/runs", a.listRuns)
	view.GET("/collections/:id/events", a.events)
	view.GET("/photos/:piece", a.photo)

	admin := r.Group("/api/admin", a.requireSession, a.requireAdmin)
	admin.GET("/viewers", a.listViewers)
	admin.POST("/viewers", a.createViewer)
	admin.PUT("/viewers/:id", a.updateViewer)
	admin.DELETE("/viewers/:id", a.deleteViewer)
	admin.GET("/keys", a.listKeys)
	admin.POST("/keys", a.createKey)
	admin.DELETE("/keys/:id", a.revokeKey)
	admin.POST("/collections/:id/clear-photos", a.clearPhotos)
	admin.DELETE("/collections/:id", a.deleteCollection)
	admin.POST("/photos/clear", a.clearAllPhotos)

	// The sorter, with an API key.
	push := r.Group("/api/push", a.requireAPIKey)
	push.GET("/ping", a.ping)
	push.PUT("/collections/:id", a.pushCollection)
	push.POST("/collections/:id/clear-unknowns", a.pushClearUnknowns)
	push.PUT("/runs/:id", a.pushRun)
	push.PUT("/pieces/:id", a.pushPiece)
	push.PUT("/pieces/:id/photo", a.pushPhoto)

	r.NoRoute(ui)
}
