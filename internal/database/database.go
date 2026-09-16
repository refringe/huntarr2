// Package database manages the SQLite database connection and migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"

	// Pure Go SQLite driver; no CGO required.
	_ "modernc.org/sqlite"
)

// Open creates a SQLite connection at the given file path, configuring pragmas for WAL mode, foreign key
// enforcement, and a busy timeout. The returned *sql.DB is safe for concurrent use.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	ctx := context.Background()

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

	// A single shared connection serialises writes and keeps the per-connection pragmas active.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
