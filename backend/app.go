package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"goklippy/backend/config"
	"goklippy/backend/logic"
	"goklippy/backend/logic/applog"
	"goklippy/backend/logic/klipperconfig"
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

	// Logs go to the console and, for Diagnose › Logs, to hourly files next to the
	// database, kept 24 h.
	logStore, err := applog.Open(filepath.Join(filepath.Dir(cfg.Database.Path), "logs"), 24*time.Hour)
	if err != nil {
		slog.Error("failed to open the log folder", "error", err)
		return
	}
	defer logStore.Close()
	slog.SetDefault(slog.New(applog.Tee{
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}),
		logStore.Handler(slog.LevelInfo),
	}))
	routes.InitLogs(logStore)

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
	machine, err := startSorter(sorterCtx, cfg.Sorter, db)
	if err != nil {
		slog.Error("failed to start the sorter", "error", err)
		return
	}

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

	// Ctrl+C or a stop request shuts down in order: the HTTP server, then the sorter,
	// whose loop stops the motors and turns the light off before the process exits.
	stopSignal, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListening()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	slog.Info("backend listening", "port", cfg.Port)

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server failure", "error", err)
		}
	case <-stopSignal.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		cancel()
	}

	stopSorter()
	select {
	case <-machine.Done():
		slog.Info("sorter stopped")
	case <-time.After(5 * time.Second):
		slog.Warn("sorter did not stop in time")
	}
}
