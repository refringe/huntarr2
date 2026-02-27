// Package database manages the SQLite database connection and migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"

	// Pure Go SQLite driver; no CGO required.
	_ "modernc.org/sqlite"
)

// Open creates a new SQLite database connection at the given file path and
// configures pragmas for WAL mode, foreign key enforcement, and a sensible
// busy timeout. The returned *sql.DB is safe for concurrent use.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	ctx := context.Background()

	// WAL mode allows concurrent reads while a write is in progress.
	// foreign_keys must be enabled per-connection in SQLite.
	// busy_timeout prevents immediate SQLITE_BUSY errors under contention.
	// synchronous=NORMAL is safe with WAL and reduces fsync overhead.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("setting pragma %q: %w", p, err)
		}
	}

	// SQLite serialises writes internally. Limiting to one open connection
	// for writes avoids lock contention; WAL mode still permits concurrent
	// readers. With database/sql, SetMaxOpenConns(1) ensures all
	// operations share a single connection, which keeps foreign_keys=ON
	// active (pragmas are per-connection).
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
