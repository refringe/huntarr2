package settings

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence operations for settings.
type Repository interface {
	ListGlobal(ctx context.Context) ([]Setting, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]Setting, error)
	Upsert(ctx context.Context, s *Setting) error
	// UpsertBatch atomically creates or updates multiple settings in a
	// single database transaction. All settings succeed or none do.
	UpsertBatch(ctx context.Context, settings []Setting) error
	Delete(ctx context.Context, instanceID *uuid.UUID, key string) error
	// DeleteBatch atomically removes multiple settings in a single
	// database transaction. All deletes succeed or none do.
	DeleteBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error
}
