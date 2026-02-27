package cooldown

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/refringe/huntarr2/internal/database/testdb"
	"github.com/refringe/huntarr2/internal/instance"
)

func TestSQLiteFilterCoolingDownEmpty(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "empty-filter-test",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://test:8989",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	got, err := repo.FilterCoolingDown(ctx, inst.ID, nil, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error for nil input: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}

	got, err = repo.FilterCoolingDown(ctx, inst.ID, []int{}, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error for empty slice: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

func TestSQLiteRecordAndFilter(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "record-filter-test",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://test:8989",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	items := []int{101, 102, 103}
	if err := repo.RecordSearches(ctx, inst.ID, items); err != nil {
		t.Fatalf("RecordSearches: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(
		ctx, inst.ID, items, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 3 {
		t.Errorf("expected 3 cooling down, got %d", len(coolingDown))
	}

	coolingDown, err = repo.FilterCoolingDown(
		ctx, inst.ID, []int{101, 999}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown subset: %v", err)
	}
	if len(coolingDown) != 1 {
		t.Errorf("expected 1 cooling down from subset, got %d",
			len(coolingDown))
	}
}

func TestSQLiteFilterCoolingDownExpired(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "expired-cooldown-test",
		AppType:   instance.AppTypeRadarr,
		BaseURL:   "http://test:7878",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	items := []int{201, 202}
	if err := repo.RecordSearches(ctx, inst.ID, items); err != nil {
		t.Fatalf("RecordSearches: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(
		ctx, inst.ID, items, 0)
	if err != nil {
		t.Fatalf("FilterCoolingDown: %v", err)
	}
	if len(coolingDown) != 0 {
		t.Errorf("expected 0 cooling down with zero duration, got %d",
			len(coolingDown))
	}
}

func TestSQLiteFilterCoolingDownInstanceIsolation(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)

	inst1 := instance.Instance{
		Name:      "isolation-inst-1",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://sonarr1:8989",
		APIKey:    "key-1",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst1); err != nil {
		t.Fatalf("creating instance 1: %v", err)
	}

	inst2 := instance.Instance{
		Name:      "isolation-inst-2",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://sonarr2:8989",
		APIKey:    "key-2",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst2); err != nil {
		t.Fatalf("creating instance 2: %v", err)
	}

	if err := repo.RecordSearches(ctx, inst1.ID, []int{301}); err != nil {
		t.Fatalf("RecordSearches for instance 1: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(
		ctx, inst2.ID, []int{301}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown for instance 2: %v", err)
	}
	if len(coolingDown) != 0 {
		t.Errorf(
			"expected 0 cooling down for different instance, got %d",
			len(coolingDown))
	}

	coolingDown, err = repo.FilterCoolingDown(
		ctx, inst1.ID, []int{301}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown for instance 1: %v", err)
	}
	if len(coolingDown) != 1 {
		t.Errorf("expected 1 cooling down for instance 1, got %d",
			len(coolingDown))
	}
}

func TestSQLiteRecordSearchesUpdatesTimestamp(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "upsert-timestamp-test",
		AppType:   instance.AppTypeLidarr,
		BaseURL:   "http://test:8686",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	if err := repo.RecordSearches(ctx, inst.ID, []int{401}); err != nil {
		t.Fatalf("initial RecordSearches: %v", err)
	}
	twoHoursAgo := time.Now().UTC().Add(-2 * time.Hour)
	_, err := db.ExecContext(ctx,
		`UPDATE search_cooldowns
		    SET searched_at = ?
		  WHERE instance_id = ? AND item_id = 401`,
		twoHoursAgo.Format(time.RFC3339Nano), inst.ID.String())
	if err != nil {
		t.Fatalf("back-dating cooldown record: %v", err)
	}

	coolingDown, err := repo.FilterCoolingDown(
		ctx, inst.ID, []int{401}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown before re-record: %v", err)
	}
	if len(coolingDown) != 0 {
		t.Fatalf(
			"expected 0 cooling down (expired), got %d", len(coolingDown))
	}

	if err := repo.RecordSearches(ctx, inst.ID, []int{401}); err != nil {
		t.Fatalf("re-RecordSearches: %v", err)
	}

	coolingDown, err = repo.FilterCoolingDown(
		ctx, inst.ID, []int{401}, time.Hour)
	if err != nil {
		t.Fatalf("FilterCoolingDown after re-record: %v", err)
	}
	if len(coolingDown) != 1 {
		t.Errorf(
			"expected 1 cooling down after re-record, got %d",
			len(coolingDown))
	}
}
