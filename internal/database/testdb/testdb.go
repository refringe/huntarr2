// Package testdb provides a lightweight SQLite database for tests. No Docker
// containers or external services are required.
package testdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/refringe/huntarr2/internal/database"
)

// New creates a temporary SQLite database in a test-scoped directory, runs
// all migrations, and returns the open *sql.DB. The database file is
// automatically removed when the test completes.
func New(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("closing test database: %v", err)
		}
	})

	if err := database.Migrate(db, zerolog.Nop()); err != nil {
		t.Fatalf("running test migrations: %v", err)
	}

	return db
}
