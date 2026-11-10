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
	klipper.PUT("/sections/*name", routes.UpdateKlipperConfigSection)
	klipper.DELETE("/sections/*name", routes.DeleteKlipperConfigSection)
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
	mcuGroup.POST("/sessions/:id/pca9685/check", routes.CheckMCUPCA9685)
	mcuGroup.POST("/sessions/:id/reset", routes.ResetMCU)
	mcuGroup.POST("/sessions/:id/stepper/jog", routes.JogMCUStepper)
	mcuGroup.POST("/sessions/:id/stepper/routine", routes.StartMCUStepperRoutine)
	mcuGroup.POST("/sessions/:id/stepper/index", routes.IndexMCUStepper)
	mcuGroup.POST("/sessions/:id/stepper/stop", routes.StopMCUStepper)

	sorterGroup := api.Group("/sorter")
	sorterGroup.GET("/settings/:name", routes.GetSorterSettings)
	sorterGroup.PUT("/settings/:name", routes.PutSorterSettings)
	sorterGroup.GET("/state", routes.GetSorterState)
	sorterGroup.POST("/commands", routes.SorterCommand)
	sorterGroup.POST("/channels/:channel/move", routes.MoveSorterChannel)
	sorterGroup.GET("/ws", routes.SorterWebSocket)
	sorterGroup.GET("/chute", routes.GetSorterChute)
	sorterGroup.POST("/chute/aim", routes.AimSorterChute)
	sorterGroup.POST("/chute/zero", routes.ZeroSorterChute)
	sorterGroup.POST("/chute/calibrate", routes.CalibrateSorterChute)
	sorterGroup.POST("/distribution/send", routes.SendToSorterBin)
	sorterGroup.POST("/distribution/test", routes.TestSorterLayer)
	sorterGroup.POST("/jobs/cancel", routes.CancelSorterJob)

	registerFrontendHosting(r, cfg.Frontend)
}
