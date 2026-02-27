// Huntarr2 automates quality upgrade searches across *arr applications.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/config"
	"github.com/refringe/huntarr2/internal/database"
	"github.com/refringe/huntarr2/internal/server"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("startup failed")
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	zerolog.SetGlobalLevel(cfg.LogLevel)

	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("built", date).
		Msg("starting huntarr2")

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn().Err(err).Msg("closing database")
		}
	}()

	if err := database.Migrate(db, log.Logger); err != nil {
		return fmt.Errorf("running database migrations: %w", err)
	}

	srv, err := server.New(cfg, db)
	if err != nil {
		return fmt.Errorf("initialising server: %w", err)
	}
	return srv.Run()
}
