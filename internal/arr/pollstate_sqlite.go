package arr

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PollTracker abstracts poll state persistence so consumers can be tested
// without a database.
type PollTracker interface {
	LastPolled(ctx context.Context, instanceID uuid.UUID) (time.Time, error)
	RecordPoll(ctx context.Context, instanceID uuid.UUID, polledAt time.Time) error
}

// SQLitePollTracker implements PollTracker using SQLite, persisted in the
// history_poll_state table.
type SQLitePollTracker struct {
	db *sql.DB
}

// NewSQLitePollTracker returns a SQLitePollTracker backed by db.
func NewSQLitePollTracker(db *sql.DB) *SQLitePollTracker {
	return &SQLitePollTracker{db: db}
}

// LastPolled returns the timestamp of the most recent history poll for the
// given instance. If the instance has never been polled, the zero time is
// returned.
func (r *SQLitePollTracker) LastPolled(ctx context.Context, instanceID uuid.UUID) (time.Time, error) {
	const q = `SELECT last_polled
	             FROM history_poll_state
	            WHERE instance_id = ?`

	var raw string
	err := r.db.QueryRowContext(ctx, q, instanceID.String()).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf(
			"querying poll state for %s: %w", instanceID, err)
	}

	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing last_polled: %w", err)
	}
	return t, nil
}

// RecordPoll upserts the last polled timestamp for the given instance.
func (r *SQLitePollTracker) RecordPoll(ctx context.Context, instanceID uuid.UUID, polledAt time.Time) error {
	const q = `
		INSERT INTO history_poll_state (instance_id, last_polled)
		VALUES (?, ?)
		ON CONFLICT (instance_id)
		DO UPDATE SET last_polled = excluded.last_polled`

	if _, err := r.db.ExecContext(ctx, q,
		instanceID.String(),
		polledAt.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("recording poll state for %s: %w",
			instanceID, err)
	}
	return nil
}
