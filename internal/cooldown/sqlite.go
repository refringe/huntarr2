package cooldown

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SQLiteRepository implements Repository using a SQLite database.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository returns a SQLiteRepository backed by db.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// FilterCoolingDown returns item IDs from the provided list that were
// searched within the cooldown period and should be excluded from the next
// search.
func (r *SQLiteRepository) FilterCoolingDown(
	ctx context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
	cooldownPeriod time.Duration,
) ([]int, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	// Build an IN clause with one placeholder per item ID.
	placeholders := make([]string, len(itemIDs))
	args := make([]any, 0, len(itemIDs)+2)
	args = append(args, instanceID.String())
	for i, id := range itemIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, int64(cooldownPeriod.Seconds()))

	//nolint:gosec // placeholders are generated literals ("?"), not user input
	query := fmt.Sprintf(
		`SELECT item_id
		   FROM search_cooldowns
		  WHERE instance_id = ?
		    AND item_id IN (%s)
		    AND searched_at > strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ', 'now', '-' || CAST(? AS TEXT) || ' seconds')`,
		strings.Join(placeholders, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying items on cooldown: %w", err)
	}
	defer rows.Close() //nolint:errcheck // checked via rows.Err below

	coolingDown := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning cooldown item: %w", err)
		}
		coolingDown = append(coolingDown, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cooldown rows: %w", err)
	}
	return coolingDown, nil
}

// RecordSearches upserts cooldown records for the given item IDs. If a
// record already exists for an (instance, item) pair, its searched_at is
// updated to now().
func (r *SQLiteRepository) RecordSearches(
	ctx context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
) error {
	if len(itemIDs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO search_cooldowns (instance_id, item_id, searched_at)
		 VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		 ON CONFLICT (instance_id, item_id)
		 DO UPDATE SET searched_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing cooldown upsert: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // best-effort cleanup

	instID := instanceID.String()
	for _, itemID := range itemIDs {
		if _, err := stmt.ExecContext(ctx, instID, itemID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording search cooldown: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing cooldown records: %w", err)
	}
	return nil
}

// DeleteExpired removes cooldown records whose searched_at is older than
// the given duration relative to now.
func (r *SQLiteRepository) DeleteExpired(
	ctx context.Context,
	olderThan time.Duration,
) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM search_cooldowns
		  WHERE searched_at < strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-' || CAST(? AS TEXT) || ' seconds')`,
		int64(olderThan.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("deleting expired cooldowns: %w", err)
	}
	return result.RowsAffected()
}
