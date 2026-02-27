package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/refringe/huntarr2/internal/database"
	"github.com/refringe/huntarr2/internal/database/testdb"
)

func TestWithTxCommits(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	err := database.WithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO instances
			 (id, name, app_type, base_url, api_key_enc, timeout_ms)
			 VALUES ('a', 'test', 'sonarr', 'http://x', 'enc', 5000)`)
		return err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM instances").Scan(&count); err != nil {
		t.Fatalf("querying instances: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (transaction should have committed)",
			count)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	fnErr := errors.New("operation failed")
	err := database.WithTx(ctx, db, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO instances
			 (id, name, app_type, base_url, api_key_enc, timeout_ms)
			 VALUES ('b', 'test', 'sonarr', 'http://x', 'enc', 5000)`)
		return fnErr
	})

	if !errors.Is(err, fnErr) {
		t.Fatalf("err = %v, want %v", err, fnErr)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM instances").Scan(&count); err != nil {
		t.Fatalf("querying instances: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (transaction should have rolled back)",
			count)
	}
}

func TestWithTxNoOpSucceeds(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)

	err := database.WithTx(context.Background(), db, func(_ *sql.Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error on no-op transaction: %v", err)
	}
}
