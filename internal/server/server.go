// Package server configures and runs the HTTP server, page handlers, and
// middleware.
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/api"
	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/config"
	"github.com/refringe/huntarr2/internal/cooldown"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/internal/scheduler"
	"github.com/refringe/huntarr2/internal/settings"
	"github.com/refringe/huntarr2/web/static"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	instances    *instance.Service
	arr          *arr.Service
	activity     *activity.Service
	scheduler    *scheduler.Scheduler
	assetVersion string
	server       *http.Server
}

// New creates a Server with all routes and middleware wired up. The commit
// parameter is the build's git commit hash, used for cache-busting static
// asset URLs.
func New(cfg *config.Config, db *sql.DB, commit string) (*Server, error) {
	instanceRepo := instance.NewSQLiteRepository(db, cfg.EncryptionKey)
	instanceSvc := instance.NewService(instanceRepo)
	arrSvc := arr.NewService(instanceRepo)

	settingsRepo := settings.NewSQLiteRepository(db)
	settingsSvc := settings.NewService(settingsRepo)

	cooldownRepo := cooldown.NewSQLiteRepository(db)

	activityRepo := activity.NewSQLiteRepository(db)
	activitySvc := activity.NewService(activityRepo)

	pollTracker := arr.NewSQLitePollTracker(db)

	sched, err := scheduler.New(
		time.Duration(cfg.SchedulerTickSecs)*time.Second,
		instanceRepo, settingsSvc, cooldownRepo,
		activitySvc, arrSvc, pollTracker,
	)
	if err != nil {
		return nil, fmt.Errorf("creating scheduler: %w", err)
	}

	apiRouter := api.NewRouter(instanceSvc, arrSvc, settingsSvc, activitySvc, sched)

	mux := http.NewServeMux()

	s := &Server{
		instances:    instanceSvc,
		arr:          arrSvc,
		activity:     activitySvc,
		scheduler:    sched,
		assetVersion: commit,
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			Handler:           withMiddleware(mux, cfg.AuthUsername, cfg.AuthPassword),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}

	s.registerRoutes(mux, apiRouter)
	return s, nil
}

// Run starts the HTTP server and scheduler, then blocks until a SIGINT or
// SIGTERM is received. On shutdown, the scheduler is stopped first, then
// the HTTP server is drained with a 30 second timeout.
func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	schedCtx, schedCancel := context.WithCancel(ctx)
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		if err := s.scheduler.Run(schedCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error().Err(err).Msg("scheduler error")
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", s.server.Addr).Msg("listening")
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			schedCancel()
			<-schedDone
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
		log.Info().Msg("shutting down gracefully")
	}

	schedCancel()
	<-schedDone
	log.Info().Msg("scheduler stopped")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	log.Info().Msg("server stopped")
	return nil
}

func (s *Server) registerRoutes(mux *http.ServeMux, apiRouter *api.Router) {
	mux.Handle("/api/", http.StripPrefix("/api", apiRouter.Handler()))

	mux.Handle("GET /static/", withStaticCacheHeaders(http.StripPrefix("/static/",
		http.FileServer(http.FS(static.FS)))))

	mux.HandleFunc("GET /{$}", s.handleHomePage)
	mux.HandleFunc("GET /connections", s.handleConnectionsPage)
	mux.HandleFunc("GET /settings", s.handleSettingsPage)
	mux.HandleFunc("GET /logs", s.handleLogsPage)
}
