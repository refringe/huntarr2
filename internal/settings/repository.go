package settings

import (
	"context"
	"uuid"
)

// Repository defines the persistence operations for settings.
type Repository interface {
	ListGlobal(ctx context.Context) ([]Setting, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]Setting, error)
	// UpsertBatch atomically creates or updates multiple settings in a single database transaction.
	UpsertBatch(ctx context.Context, settings []Setting) error
	// DeleteBatch atomically removes multiple settings in a single database transaction.
	DeleteBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error
}
