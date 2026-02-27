package instance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/encrypt"
)

// SQLiteRepository implements Repository using a SQLite database. API keys
// are encrypted at rest using AES-256-GCM.
type SQLiteRepository struct {
	db            *sql.DB
	encryptionKey []byte
}

// NewSQLiteRepository returns a SQLiteRepository backed by db. The
// encryptionKey must be exactly 32 bytes and is used to encrypt and decrypt
// API keys stored in the database. It panics if encryptionKey is not the
// required length because an invalid key represents a configuration error
// that must be caught at startup rather than at query time.
func NewSQLiteRepository(db *sql.DB, encryptionKey []byte) *SQLiteRepository {
	if len(encryptionKey) != 32 {
		panic("instance.NewSQLiteRepository: encryptionKey must be exactly 32 bytes")
	}
	return &SQLiteRepository{db: db, encryptionKey: encryptionKey}
}

// List returns all instances ordered by name.
func (r *SQLiteRepository) List(ctx context.Context) ([]Instance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, app_type, base_url, api_key_enc,
		        timeout_ms, created_at, updated_at
		   FROM instances
		  ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying instances: %w", err)
	}

	return r.collectInstances(rows)
}

// ListByType returns all instances of the given application type ordered by
// name.
func (r *SQLiteRepository) ListByType(ctx context.Context, appType AppType) ([]Instance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, app_type, base_url, api_key_enc,
		        timeout_ms, created_at, updated_at
		   FROM instances
		  WHERE app_type = ?
		  ORDER BY name`, string(appType))
	if err != nil {
		return nil, fmt.Errorf("querying instances by type: %w", err)
	}

	return r.collectInstances(rows)
}

// Get returns the instance with the given ID. It returns ErrNotFound when
// no matching row exists.
func (r *SQLiteRepository) Get(ctx context.Context, id uuid.UUID) (Instance, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, app_type, base_url, api_key_enc,
		        timeout_ms, created_at, updated_at
		   FROM instances
		  WHERE id = ?`, id.String())

	inst, err := r.scanInstance(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Instance{}, ErrNotFound
		}
		return Instance{}, fmt.Errorf("querying instance %s: %w", id, err)
	}
	return inst, nil
}

// Create inserts a new instance. If inst.ID is the zero value, a UUID is
// generated in Go. Timestamps are set to the current time.
func (r *SQLiteRepository) Create(ctx context.Context, inst *Instance) error {
	encryptedKey, err := encrypt.Encrypt(inst.APIKey, r.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}

	now := time.Now().UTC()
	inst.CreatedAt = now
	inst.UpdatedAt = now

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO instances
		        (id, name, app_type, base_url, api_key_enc,
		         timeout_ms, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID.String(), inst.Name, string(inst.AppType),
		inst.BaseURL, encryptedKey, inst.TimeoutMs,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("creating instance: %w", err)
	}
	return nil
}

// Update persists changes to an existing instance.
func (r *SQLiteRepository) Update(ctx context.Context, inst *Instance) error {
	encryptedKey, err := encrypt.Encrypt(inst.APIKey, r.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE instances
		    SET name = ?, base_url = ?, api_key_enc = ?,
		        timeout_ms = ?, updated_at = ?
		  WHERE id = ?`,
		inst.Name, inst.BaseURL, encryptedKey,
		inst.TimeoutMs, now.Format(time.RFC3339Nano), inst.ID.String())
	if err != nil {
		return fmt.Errorf("updating instance %s: %w", inst.ID, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	inst.UpdatedAt = now
	return nil
}

// Delete removes the instance with the given ID. It returns ErrNotFound
// when no matching row exists.
func (r *SQLiteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM instances WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting instance %s: %w", id, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanInstance reads a single Instance from a *sql.Row and decrypts the
// API key.
func (r *SQLiteRepository) scanInstance(row *sql.Row) (Instance, error) {
	var inst Instance
	var idStr, appType, createdAt, updatedAt string
	if err := row.Scan(
		&idStr, &inst.Name, &appType, &inst.BaseURL,
		&inst.APIKey, &inst.TimeoutMs, &createdAt, &updatedAt,
	); err != nil {
		return Instance{}, err
	}

	return r.hydrateInstance(inst, idStr, appType, createdAt, updatedAt)
}

// collectInstances reads all rows into a slice of Instance values,
// decrypting each API key.
func (r *SQLiteRepository) collectInstances(rows *sql.Rows) ([]Instance, error) {
	defer rows.Close() //nolint:errcheck // checked via rows.Err below

	var instances []Instance
	for rows.Next() {
		var inst Instance
		var idStr, appType, createdAt, updatedAt string
		if err := rows.Scan(
			&idStr, &inst.Name, &appType, &inst.BaseURL,
			&inst.APIKey, &inst.TimeoutMs, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning instance row: %w", err)
		}

		hydrated, err := r.hydrateInstance(
			inst, idStr, appType, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		instances = append(instances, hydrated)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating instance rows: %w", err)
	}

	return instances, nil
}

// hydrateInstance parses string fields from SQLite into typed fields and
// decrypts the API key.
func (r *SQLiteRepository) hydrateInstance(
	inst Instance,
	idStr, appType, createdAt, updatedAt string,
) (Instance, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Instance{}, fmt.Errorf("parsing instance ID: %w", err)
	}
	inst.ID = id
	inst.AppType = AppType(appType)

	inst.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Instance{}, fmt.Errorf("parsing created_at: %w", err)
	}
	inst.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Instance{}, fmt.Errorf("parsing updated_at: %w", err)
	}

	decrypted, err := encrypt.Decrypt(inst.APIKey, r.encryptionKey)
	if err != nil {
		return Instance{}, fmt.Errorf("decrypting API key for %s: %w",
			inst.ID, err)
	}
	inst.APIKey = decrypted

	return inst, nil
}
