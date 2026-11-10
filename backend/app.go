package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"goklippy/backend/config"
	"goklippy/backend/logic"
	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/klipperconfig"
	"goklippy/backend/logic/sorter"
	"goklippy/backend/mcu"
	"goklippy/backend/models"
	"goklippy/backend/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	configPath := flag.String("config", "../private/config-dev.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "path", *configPath, "error", err)
		return
	}

	db, err := models.OpenDB(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open database", "path", cfg.Database.Path, "error", err)
		return
	}
	defer db.Close()
	if err := models.ApplySchemaFile(context.Background(), db, cfg.Database.SchemaPath); err != nil {
		slog.Error("failed to apply database schema", "error", err)
		return
	}
	slog.Info("database ready", "path", cfg.Database.Path)

	created, err := models.EnsureInitialUser(context.Background(), db, models.InitialUserSeed{
		Enabled:  cfg.InitialUser.Enabled,
		Email:    cfg.InitialUser.Email,
		Password: cfg.InitialUser.Password,
		Role:     cfg.InitialUser.Role,
	})
	if err != nil {
		slog.Error("failed to ensure initial user", "error", err)
		return
	}
	if created {
		slog.Info("initial user created", "email", cfg.InitialUser.Email)
	}

	authStore := models.NewAuthStore(db)

	authService := logic.NewAuthService(authStore, logic.AuthConfig{
		AccessSecret:    cfg.JWT.AccessSecret,
		RefreshSecret:   cfg.JWT.RefreshSecret,
		AccessTTL:       time.Duration(cfg.JWT.AccessTTLMin) * time.Minute,
		RefreshTokenTTL: time.Duration(cfg.JWT.RefreshTTLHour) * time.Hour,
	})
	routes.InitAuthService(authService)

	if cfg.Klipper.ConfigRoot != "" {
		cfgSvc, err := klipperconfig.NewService(cfg.Klipper.ConfigRoot)
		if err != nil {
			slog.Error("failed to init klipper config service", "error", err)
			return
		}
		routes.InitKlipperConfigService(cfgSvc)
		if err := cfgSvc.EnsureDefault(); err != nil {
			slog.Warn("no default klipper config loaded", "error", err)
		} else if ws, err := cfgSvc.Current(); err == nil {
			slog.Info("klipper config loaded", "path", ws.Path())
		}
		slog.Info("klipper config root", "path", cfg.Klipper.ConfigRoot)
	}
	routes.InitBoardProfilesDir(filepath.Clean(cfg.Klipper.ProfilesDir))
	routes.InitMCUConfig(cfg.MCU)

	sorterCtx, stopSorter := context.WithCancel(context.Background())
	defer stopSorter()
	sorterSettings := sorter.NewSettingsService(sorterCtx, models.NewSorterSettingsStore(db))
	var sorterHW sorter.Hardware = sorter.NewMCUHardware(mcu.DefaultManager(), sorterSettings.Machine)
	if cfg.Sorter.Simulate {
		sorterHW = sorter.NewSimHardware(sorterSettings.Machine)
		slog.Info("sorter running on simulated hardware")
	}
	sorterMachine := sorter.NewMachine(sorterHW, sorterSettings.Machine, 0)
	go sorterMachine.Run(sorterCtx)
	routes.InitSorter(sorterMachine, sorterSettings)
	routes.InitSorting(sorter.NewSortingService(sorterCtx, models.NewSortingStore(db), sorterSettings.Machine), brickognize.NewClient())

	r := gin.New()
	r.Use(routes.SlogRecovery())
	r.Use(routes.RequestContextMiddleware())
	r.Use(routes.SlogRequestLogger())
	r.Use(routes.CORSMiddleware())
	registerRoutes(r, cfg)

	// MCU connect/configure can take 20–60s (serial boot wait + config + retry).
	const serverWriteTimeout = 120 * time.Second

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("backend listening", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failure", "error", err)
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
