package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"goklippy/backend/config"
	"goklippy/backend/logic"
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

	if cfg.DatabaseURL() == "" {
		slog.Error("database configuration is required")
		return
	}

	db, err := models.OpenDB(cfg.DatabaseURL())
	if err != nil {
		slog.Error("failed to initialize database connection", "error", err)
		return
	}
	if db == nil {
		slog.Error("database configuration is required")
		return
	}
	defer db.Close()

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

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(routes.RequestContextMiddleware())
	r.Use(routes.SlogRequestLogger())
	r.Use(routes.CORSMiddleware())
	registerRoutes(r, cfg)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
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
