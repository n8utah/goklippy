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

	klipper := api.Group("/klipper-config")
	klipper.GET("/files", routes.ListKlipperConfigFiles)
	klipper.POST("/load", routes.LoadKlipperConfig)
	klipper.GET("/workspace", routes.GetKlipperConfigWorkspace)
	klipper.PUT("/sections/:name", routes.UpdateKlipperConfigSection)
	klipper.DELETE("/sections/:name", routes.DeleteKlipperConfigSection)
	klipper.PUT("/raw", routes.UpdateKlipperConfigRaw)
	klipper.POST("/save", routes.SaveKlipperConfig)

	boards := api.Group("/board-profiles")
	boards.GET("", routes.ListBoardProfiles)
	boards.GET("/:id", routes.GetBoardProfile)
	boards.GET("/:id/diagram", routes.GetBoardProfileDiagram)
	boards.GET("/:id/image", routes.ServeBoardProfileImage)
	boards.GET("/:id/editor", routes.GetBoardProfileEditor)
	boards.PUT("/:id/hotspots", routes.UpdateBoardProfileHotspots)

	mcuGroup := api.Group("/mcu")
	mcuGroup.GET("/ports", routes.GetMCUPorts)
	mcuGroup.POST("/scan", routes.ScanMCUPorts)
	mcuGroup.POST("/connect", routes.ConnectMCU)
	mcuGroup.POST("/disconnect", routes.DisconnectMCU)
	mcuGroup.GET("/sessions", routes.ListMCUSessions)
	mcuGroup.GET("/sessions/:id", routes.GetMCUSession)
	mcuGroup.GET("/sessions/:id/ws", routes.MCUSessionWebSocket)
	mcuGroup.GET("/sessions/:id/readings", routes.GetMCUReadings)
	mcuGroup.POST("/sessions/:id/digital-out", routes.SetMCUDigitalOut)
	mcuGroup.POST("/sessions/:id/servo", routes.SetMCUServo)
	mcuGroup.POST("/sessions/:id/stepper/jog", routes.JogMCUStepper)
	mcuGroup.POST("/sessions/:id/stepper/stop", routes.StopMCUStepper)

	registerFrontendHosting(r, cfg.Frontend)
}
