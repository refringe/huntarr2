package database

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"

	"github.com/refringe/huntarr2/internal/database/migrations"
)

// migrateMu serialises calls to Migrate so that concurrent tests do not
// race on goose's package-level state (SetBaseFS, SetDialect, SetLogger).
// Each test database is independent, so the serialisation only affects the
// brief migration setup, not ongoing queries.
var migrateMu sync.Mutex

// Migrate runs all pending database migrations using goose. The provided
// *sql.DB must already be open and configured.
func Migrate(db *sql.DB, logger zerolog.Logger) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(&gooseLogger{logger: logger})

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

// gooseLogger adapts zerolog to the goose.Logger interface.
type gooseLogger struct {
	logger zerolog.Logger
}

func (g *gooseLogger) Fatalf(format string, v ...any) {
	g.logger.Fatal().Msgf(format, v...)
}

func (g *gooseLogger) Printf(format string, v ...any) {
	g.logger.Info().Msgf(format, v...)
}
