package cooldown

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeRepository implements Repository in memory for testing the interface
// contract.
type fakeRepository struct {
	mu      sync.Mutex
	records map[uuid.UUID]map[int]time.Time
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{records: make(map[uuid.UUID]map[int]time.Time)}
}

func (f *fakeRepository) FilterCoolingDown(
	_ context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
	cooldownPeriod time.Duration,
) ([]int, error) {
	if len(itemIDs) == 0 {
		return []int{}, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	cutoff := time.Now().Add(-cooldownPeriod)
	instRecords := f.records[instanceID]
	coolingDown := make([]int, 0)
	for _, id := range itemIDs {
		if searchedAt, ok := instRecords[id]; ok && searchedAt.After(cutoff) {
			coolingDown = append(coolingDown, id)
		}
	}
	return coolingDown, nil
}

func (f *fakeRepository) RecordSearches(
	_ context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
) error {
	if len(itemIDs) == 0 {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.records[instanceID] == nil {
		f.records[instanceID] = make(map[int]time.Time)
	}
	now := time.Now()
	for _, id := range itemIDs {
		f.records[instanceID][id] = now
	}
	return nil
}

func (f *fakeRepository) DeleteExpired(
	_ context.Context,
	olderThan time.Duration,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var deleted int64
	for instID, items := range f.records {
		for itemID, searchedAt := range items {
			if searchedAt.Before(cutoff) {
				delete(items, itemID)
				deleted++
			}
		}
		if len(items) == 0 {
			delete(f.records, instID)
		}
	}
	return deleted, nil
}

func TestFilterCoolingDownEmpty(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()

	got, err := repo.FilterCoolingDown(
		context.Background(), uuid.New(), nil, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestRecordAndFilter(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	ctx := context.Background()
	instID := uuid.New()

	items := []int{101, 102, 103}
	if err := repo.RecordSearches(ctx, instID, items); err != nil {
		t.Fatalf("RecordSearches: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(ctx, instID, items, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 3 {
		t.Errorf("expected 3 cooling down, got %d", len(coolingDown))
	}
}

func TestFilterCoolingDownExpired(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	ctx := context.Background()
	instID := uuid.New()

	// Manually insert a record with an old timestamp.
	repo.mu.Lock()
	repo.records[instID] = map[int]time.Time{
		101: time.Now().Add(-2 * time.Hour),
		102: time.Now(),
	}
	repo.mu.Unlock()

	coolingDown, err := repo.FilterCoolingDown(
		ctx, instID, []int{101, 102}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 1 {
		t.Fatalf("expected 1 cooling down, got %d", len(coolingDown))
	}
	if coolingDown[0] != 102 {
		t.Errorf("expected item 102 cooling down, got %d", coolingDown[0])
	}
}

func TestFilterCoolingDownSeparatesInstances(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	ctx := context.Background()

	inst1 := uuid.New()
	inst2 := uuid.New()

	if err := repo.RecordSearches(ctx, inst1, []int{101}); err != nil {
		t.Fatalf("RecordSearches: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(
		ctx, inst2, []int{101}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 0 {
		t.Errorf("expected 0 cooling down for different instance, got %d",
			len(coolingDown))
	}
}

func TestRecordSearchesEmpty(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()

	err := repo.RecordSearches(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordSearchesUpdatesTimestamp(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	ctx := context.Background()
	instID := uuid.New()

	// Record with an old timestamp.
	repo.mu.Lock()
	repo.records[instID] = map[int]time.Time{
		101: time.Now().Add(-2 * time.Hour),
	}
	repo.mu.Unlock()

	// Verify it has expired.
	coolingDown, err := repo.FilterCoolingDown(
		ctx, instID, []int{101}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 0 {
		t.Fatalf("expected 0 cooling down (expired), got %d", len(coolingDown))
	}

	// Re-record the search, which should update the timestamp.
	if err := repo.RecordSearches(ctx, instID, []int{101}); err != nil {
		t.Fatalf("RecordSearches: %v", err)
	}

	// Now it should be cooling down again.
	coolingDown, err = repo.FilterCoolingDown(
		ctx, instID, []int{101}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 1 {
		t.Errorf("expected 1 cooling down after re-record, got %d",
			len(coolingDown))
	}
}
