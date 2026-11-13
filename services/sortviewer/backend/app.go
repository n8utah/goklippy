// Sort viewer: shows a LEGO sort as it happens to the people whose LEGO it is. The
// sorter (goklippy) pushes runs, pieces, photos and its sorting plan here.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/config"
	"goklippy/services/sortviewer/backend/logic"
	"goklippy/services/sortviewer/backend/models"
	"goklippy/services/sortviewer/backend/routes"
	"goklippy/services/sortviewer/backend/webui"
)

// commit is the git commit, stamped by mage build (-ldflags -X main.commit=...).
var commit = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.FromEnv()

	store, err := models.Open(filepath.Join(cfg.DataDir, "sortviewer.db"))
	if err != nil {
		slog.Error("opening the database failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	svc := logic.NewService(store, cfg.DataDir)
	admin := logic.NewAdminSignIn(logic.OIDCConfig{
		Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID, ClientSecret: cfg.OIDCClientSecret,
		RedirectURL: cfg.AppURL + "/auth/callback", AdminEmails: cfg.AdminEmails,
	})
	if !admin.Enabled() {
		slog.Warn("admin sign-in through the Gangway isn't set up (OIDC_ISSUER, OIDC_CLIENT_ID)")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), routes.RequestLog())
	api := &routes.API{Svc: svc, Admin: admin, Cfg: cfg, Commit: commit}
	api.Register(r, webui.Handler())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go svc.CleanSessions(ctx)

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("sort viewer listening", "addr", srv.Addr, "url", cfg.AppURL, "commit", commit, "data", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
