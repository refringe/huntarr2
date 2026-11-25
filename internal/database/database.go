// Package database manages the SQLite database connection and migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	// Pure Go SQLite driver; no CGO required.
	_ "modernc.org/sqlite"
)

// connectionPragmas are applied to every connection the pool opens, in order.
var connectionPragmas = []string{
	"busy_timeout(5000)",
	"journal_mode(WAL)",
	"foreign_keys(ON)",
	"synchronous(NORMAL)",
}

// Open opens the SQLite database at path with WAL mode, foreign key enforcement, and a busy timeout.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?" + url.Values{"_pragma": connectionPragmas}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// A single shared connection serialises writes.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
