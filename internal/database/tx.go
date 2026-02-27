package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// WithTx executes fn within a database transaction. The transaction is
// committed if fn returns nil and rolled back otherwise.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rolling back transaction: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
