// Package testdb provides a lightweight SQLite database for tests, with no Docker or external services.
package testdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/refringe/huntarr2/internal/database"
)

// New returns a migrated SQLite database in a test-scoped temporary directory, removed when the test completes.
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
