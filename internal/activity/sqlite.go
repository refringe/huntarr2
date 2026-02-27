package activity

import (
	"context"
	"database/sql"
	"encoding/json"
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

// Create inserts a new activity log entry. The Details map is marshalled
// to JSON text.
func (r *SQLiteRepository) Create(ctx context.Context, entry *Entry) error {
	detailsJSON, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshalling activity details: %w", err)
	}

	entry.ID = uuid.New()
	entry.CreatedAt = time.Now().UTC()

	var instID *string
	if entry.InstanceID != nil {
		s := entry.InstanceID.String()
		instID = &s
	}

	var details *string
	if entry.Details != nil {
		s := string(detailsJSON)
		details = &s
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO activity_log
		        (id, instance_id, level, action, message, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID.String(), instID, string(entry.Level),
		string(entry.Action), entry.Message, details,
		entry.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("creating activity entry: %w", err)
	}
	return nil
}

// List returns activity log entries matching the given parameters, ordered
// by created_at descending.
func (r *SQLiteRepository) List(ctx context.Context, params ListParams) ([]Entry, error) {
	query, args := buildListQuery(
		`SELECT id, instance_id, level, action, message,
		        details, created_at`, params)
	query += " ORDER BY created_at DESC"
	if params.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, params.Limit)
	}
	if params.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, params.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying activity log: %w", err)
	}
	defer rows.Close() //nolint:errcheck // checked via rows.Err below

	var entries []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating activity rows: %w", err)
	}
	return entries, nil
}

// Count returns the total number of entries matching the given parameters.
func (r *SQLiteRepository) Count(ctx context.Context, params ListParams) (int, error) {
	query, args := buildListQuery("SELECT count(*)", params)

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting activity log entries: %w", err)
	}
	return count, nil
}

// Stats returns per-instance, per-action counts, optionally filtered to
// entries created at or after since.
func (r *SQLiteRepository) Stats(ctx context.Context, since *time.Time) ([]ActionStats, error) {
	var args []any
	query := `SELECT al.instance_id, COALESCE(i.name, ''),
	                 al.action, count(*) AS cnt
	            FROM activity_log al
	            LEFT JOIN instances i ON i.id = al.instance_id`
	if since != nil {
		args = append(args, since.Format(time.RFC3339Nano))
		query += " WHERE al.created_at >= ?"
	}
	query += ` GROUP BY al.instance_id, i.name, al.action
	           ORDER BY cnt DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying activity stats: %w", err)
	}
	defer rows.Close() //nolint:errcheck // checked via rows.Err below

	var results []ActionStats
	for rows.Next() {
		var s ActionStats
		var instIDStr *string
		var action string
		if err := rows.Scan(
			&instIDStr, &s.InstanceName, &action, &s.Count,
		); err != nil {
			return nil, fmt.Errorf("scanning activity stats row: %w", err)
		}
		if instIDStr != nil {
			instID, err := uuid.Parse(*instIDStr)
			if err != nil {
				return nil, fmt.Errorf("parsing instance ID: %w", err)
			}
			s.InstanceID = &instID
		}
		s.Action = Action(action)
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating activity stats rows: %w", err)
	}
	return results, nil
}

// DeleteBefore removes all activity log entries created before the given
// timestamp and returns the number of rows removed.
func (r *SQLiteRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM activity_log WHERE created_at < ?`,
		before.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("deleting old activity entries: %w", err)
	}
	return result.RowsAffected()
}

// scanEntry reads a single Entry from the current row position.
func scanEntry(rows *sql.Rows) (Entry, error) {
	var e Entry
	var idStr string
	var instIDStr *string
	var level, action string
	var detailsJSON *string
	var createdAt string

	if err := rows.Scan(
		&idStr, &instIDStr, &level, &action,
		&e.Message, &detailsJSON, &createdAt,
	); err != nil {
		return Entry{}, fmt.Errorf("scanning activity row: %w", err)
	}

	var err error
	e.ID, err = uuid.Parse(idStr)
	if err != nil {
		return Entry{}, fmt.Errorf("parsing entry ID: %w", err)
	}

	if instIDStr != nil {
		instID, err := uuid.Parse(*instIDStr)
		if err != nil {
			return Entry{}, fmt.Errorf("parsing instance ID: %w", err)
		}
		e.InstanceID = &instID
	}

	e.Level = Level(level)
	e.Action = Action(action)

	e.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Entry{}, fmt.Errorf("parsing created_at: %w", err)
	}

	if detailsJSON != nil {
		if err := json.Unmarshal([]byte(*detailsJSON), &e.Details); err != nil {
			return Entry{}, fmt.Errorf("unmarshalling activity details: %w",
				err)
		}
	}

	return e, nil
}

// buildListQuery constructs the FROM/WHERE portion of a query from the
// given params. It returns the full query string and positional arguments.
func buildListQuery(selectClause string, params ListParams) (string, []any) {
	var conditions []string
	var args []any

	if params.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, string(params.Level))
	}
	if params.InstanceID != nil {
		conditions = append(conditions, "instance_id = ?")
		args = append(args, params.InstanceID.String())
	}
	if params.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, string(params.Action))
	}
	if params.Search != "" {
		conditions = append(conditions, "message LIKE '%' || ? || '%'")
		args = append(args, params.Search)
	}
	if params.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, params.Since.Format(time.RFC3339Nano))
	}
	if params.Until != nil {
		conditions = append(conditions, "created_at < ?")
		args = append(args, params.Until.Format(time.RFC3339Nano))
	}

	query := selectClause + " FROM activity_log"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}
