package instance

import (
	"context"
	"uuid"
)

// Repository defines the persistence operations for instances.
type Repository interface {
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, id uuid.UUID) (Instance, error)
	Create(ctx context.Context, inst *Instance) error
	Update(ctx context.Context, inst *Instance) error
	Delete(ctx context.Context, id uuid.UUID) error
}
