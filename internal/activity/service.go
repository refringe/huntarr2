package activity

import (
	"context"
	"fmt"
	"time"
)

// maxListLimit caps the number of entries returned by a single List call.
const maxListLimit = 500

// Service provides operations for recording and querying activity log
// entries.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Log validates the entry level and action, then persists it.
func (s *Service) Log(ctx context.Context, entry *Entry) error {
	if !ValidLevel(entry.Level) {
		return fmt.Errorf("level %q: %w", entry.Level, ErrInvalidLevel)
	}
	if !ValidAction(entry.Action) {
		return fmt.Errorf("action %q: %w", entry.Action, ErrInvalidAction)
	}
	return s.repo.Create(ctx, entry)
}

// List returns activity log entries matching the given parameters. The limit
// is capped at maxListLimit and defaults to 50 if unset. The returned slice
// is always non-nil so that JSON serialisation produces [] rather than null.
func (s *Service) List(ctx context.Context, params ListParams) ([]Entry, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > maxListLimit {
		params.Limit = maxListLimit
	}
	entries, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

// Count returns the total number of entries matching the given parameters.
func (s *Service) Count(ctx context.Context, params ListParams) (int, error) {
	return s.repo.Count(ctx, params)
}

// Stats returns per-instance, per-action counts, optionally filtered to
// entries created at or after since.
func (s *Service) Stats(ctx context.Context, since *time.Time) ([]ActionStats, error) {
	return s.repo.Stats(ctx, since)
}

// Prune deletes activity log entries older than the given retention period
// and returns the number of rows removed.
func (s *Service) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	before := time.Now().Add(-retention)
	return s.repo.DeleteBefore(ctx, before)
}
