package activity

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ListParams controls filtering and pagination when listing activity log
// entries.
type ListParams struct {
	Level      Level
	InstanceID *uuid.UUID
	Action     Action
	Search     string
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// ActionStats holds a per-instance, per-action count returned by the Stats
// query.
type ActionStats struct {
	InstanceID   *uuid.UUID
	InstanceName string
	Action       Action
	Count        int
}

// Repository defines the persistence operations for activity log entries.
type Repository interface {
	Create(ctx context.Context, entry *Entry) error
	List(ctx context.Context, params ListParams) ([]Entry, error)
	Count(ctx context.Context, params ListParams) (int, error)
	Stats(ctx context.Context, since *time.Time) ([]ActionStats, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}
