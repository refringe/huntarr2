package settings

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/database"
)

// SQLiteRepository implements Repository using a SQLite database.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository returns a SQLiteRepository backed by db.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// ListGlobal returns all global settings (those with no instance_id)
// ordered by setting_key.
func (r *SQLiteRepository) ListGlobal(ctx context.Context) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, instance_id, setting_key, value, updated_at
		   FROM settings
		  WHERE instance_id IS NULL
		  ORDER BY setting_key`)
	if err != nil {
		return nil, fmt.Errorf("querying global settings: %w", err)
	}
	return scanSettings(rows)
}

// ListByInstance returns all settings for the given instance ordered by
// setting_key.
func (r *SQLiteRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, instance_id, setting_key, value, updated_at
		   FROM settings
		  WHERE instance_id = ?
		  ORDER BY setting_key`, instanceID.String())
	if err != nil {
		return nil, fmt.Errorf("querying instance settings: %w", err)
	}
	return scanSettings(rows)
}

// scanSettings collects Setting rows from an open *sql.Rows cursor. The
// caller does not need to close rows; scanSettings handles that.
func scanSettings(rows *sql.Rows) ([]Setting, error) {
	defer rows.Close() //nolint:errcheck // checked via rows.Err below

	var out []Setting
	for rows.Next() {
		var s Setting
		var idStr string
		var instIDStr *string
		var updatedAt string
		if err := rows.Scan(
			&idStr, &instIDStr, &s.Key, &s.Value, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning setting row: %w", err)
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("parsing setting ID: %w", err)
		}
		s.ID = id

		if instIDStr != nil {
			instID, err := uuid.Parse(*instIDStr)
			if err != nil {
				return nil, fmt.Errorf("parsing instance ID: %w", err)
			}
			s.InstanceID = &instID
		}

		s.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}

		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating setting rows: %w", err)
	}
	return out, nil
}

// Upsert creates or updates a setting. For global settings (InstanceID is
// nil), it uses the partial unique index on setting_key where instance_id
// IS NULL. For per-instance settings, it uses ON CONFLICT on the composite
// unique index.
func (r *SQLiteRepository) Upsert(ctx context.Context, s *Setting) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.New().String()

	if s.InstanceID == nil {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO settings (id, setting_key, value, updated_at)
			      VALUES (?, ?, ?, ?)
			 ON CONFLICT (setting_key) WHERE instance_id IS NULL
			 DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			id, s.Key, s.Value, now)
		if err != nil {
			return fmt.Errorf("upserting global setting %q: %w",
				s.Key, err)
		}
		return nil
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO settings
		        (id, instance_id, setting_key, value, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (instance_id, setting_key)
		     WHERE instance_id IS NOT NULL
		 DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		id, s.InstanceID.String(), s.Key, s.Value, now)
	if err != nil {
		return fmt.Errorf("upserting instance setting %q: %w", s.Key, err)
	}
	return nil
}

// execUpsert runs the upsert SQL for a single setting within an existing
// transaction.
func execUpsert(ctx context.Context, tx *sql.Tx, s *Setting) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.New().String()

	if s.InstanceID == nil {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO settings (id, setting_key, value, updated_at)
			      VALUES (?, ?, ?, ?)
			 ON CONFLICT (setting_key) WHERE instance_id IS NULL
			 DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			id, s.Key, s.Value, now)
		if err != nil {
			return fmt.Errorf("upserting global setting %q: %w",
				s.Key, err)
		}
		return nil
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO settings
		        (id, instance_id, setting_key, value, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (instance_id, setting_key)
		     WHERE instance_id IS NOT NULL
		 DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		id, s.InstanceID.String(), s.Key, s.Value, now)
	if err != nil {
		return fmt.Errorf("upserting instance setting %q: %w", s.Key, err)
	}
	return nil
}

// UpsertBatch atomically creates or updates multiple settings in a single
// database transaction.
func (r *SQLiteRepository) UpsertBatch(ctx context.Context, settings []Setting) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, s := range settings {
			if err := execUpsert(ctx, tx, &s); err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete removes a setting by key and optional instance_id.
func (r *SQLiteRepository) Delete(ctx context.Context, instanceID *uuid.UUID, key string) error {
	if instanceID == nil {
		_, err := r.db.ExecContext(ctx,
			`DELETE FROM settings
			  WHERE instance_id IS NULL AND setting_key = ?`, key)
		if err != nil {
			return fmt.Errorf("deleting global setting %q: %w", key, err)
		}
		return nil
	}

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM settings
		  WHERE instance_id = ? AND setting_key = ?`,
		instanceID.String(), key)
	if err != nil {
		return fmt.Errorf("deleting instance setting %q: %w", key, err)
	}
	return nil
}

// DeleteBatch atomically removes multiple settings in a single database
// transaction.
func (r *SQLiteRepository) DeleteBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, key := range keys {
			if instanceID == nil {
				_, err := tx.ExecContext(ctx,
					`DELETE FROM settings
					  WHERE instance_id IS NULL
					    AND setting_key = ?`, key)
				if err != nil {
					return fmt.Errorf("deleting global setting %q: %w",
						key, err)
				}
			} else {
				_, err := tx.ExecContext(ctx,
					`DELETE FROM settings
					  WHERE instance_id = ?
					    AND setting_key = ?`,
					instanceID.String(), key)
				if err != nil {
					return fmt.Errorf(
						"deleting instance setting %q: %w", key, err)
				}
			}
		}
		return nil
	})
}
