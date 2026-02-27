// Package cooldown tracks per-item search cooldowns to prevent redundant
// searches of the same item within a configurable period.
package cooldown

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the persistence operations for search cooldowns.
type Repository interface {
	// FilterCoolingDown returns the subset of itemIDs that were searched
	// within the cooldown period and should not be searched again yet.
	FilterCoolingDown(ctx context.Context, instanceID uuid.UUID,
		itemIDs []int, cooldownPeriod time.Duration) ([]int, error)

	// RecordSearches upserts cooldown records for the given item IDs, setting
	// searched_at to now().
	RecordSearches(ctx context.Context, instanceID uuid.UUID,
		itemIDs []int) error

	// DeleteExpired removes cooldown records whose searched_at is older than
	// the given duration. Returns the number of rows deleted.
	DeleteExpired(ctx context.Context, olderThan time.Duration) (int64, error)
}
