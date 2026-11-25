package database

import (
	"path/filepath"
	"testing"
)

func TestOpenAppliesPragmasToEveryConnection(t *testing.T) {
	t.Parallel()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(2)
	ctx := t.Context()

	for i := range 2 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquiring connection %d: %v", i, err)
		}
		t.Cleanup(func() { _ = conn.Close() })

		var foreignKeys, busyTimeout int
		var journalMode string
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("connection %d: reading foreign_keys: %v", i, err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("connection %d: reading busy_timeout: %v", i, err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("connection %d: reading journal_mode: %v", i, err)
		}

		if foreignKeys != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1", i, foreignKeys)
		}
		if busyTimeout != 5000 {
			t.Errorf("connection %d: busy_timeout = %d, want 5000", i, busyTimeout)
		}
		if journalMode != "wal" {
			t.Errorf("connection %d: journal_mode = %q, want wal", i, journalMode)
		}
	}
}
