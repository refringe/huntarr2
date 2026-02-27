package arr

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/refringe/huntarr2/internal/database/testdb"
	"github.com/refringe/huntarr2/internal/instance"
)

func TestLastPolledReturnsZeroForNewInstance(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	tracker := NewSQLitePollTracker(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "never-polled",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://test:8989",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	got, err := tracker.LastPolled(ctx, inst.ID)
	if err != nil {
		t.Fatalf("LastPolled: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for never-polled instance, got %v",
			got)
	}
}

func TestRecordPollThenLastPolled(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	tracker := NewSQLitePollTracker(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "poll-record-test",
		AppType:   instance.AppTypeRadarr,
		BaseURL:   "http://test:7878",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	polledAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := tracker.RecordPoll(ctx, inst.ID, polledAt); err != nil {
		t.Fatalf("RecordPoll: %v", err)
	}

	got, err := tracker.LastPolled(ctx, inst.ID)
	if err != nil {
		t.Fatalf("LastPolled: %v", err)
	}
	if !got.UTC().Truncate(time.Microsecond).Equal(polledAt) {
		t.Errorf("expected %v, got %v", polledAt, got)
	}
}

func TestRecordPollUpserts(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	tracker := NewSQLitePollTracker(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      "poll-upsert-test",
		AppType:   instance.AppTypeLidarr,
		BaseURL:   "http://test:8686",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	first := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := tracker.RecordPoll(ctx, inst.ID, first); err != nil {
		t.Fatalf("first RecordPoll: %v", err)
	}

	got, err := tracker.LastPolled(ctx, inst.ID)
	if err != nil {
		t.Fatalf("LastPolled after first record: %v", err)
	}
	if !got.UTC().Truncate(time.Microsecond).Equal(first) {
		t.Errorf("after first record: expected %v, got %v", first, got)
	}

	second := time.Date(2025, 6, 15, 18, 30, 0, 0, time.UTC)
	if err := tracker.RecordPoll(ctx, inst.ID, second); err != nil {
		t.Fatalf("second RecordPoll: %v", err)
	}

	got, err = tracker.LastPolled(ctx, inst.ID)
	if err != nil {
		t.Fatalf("LastPolled after second record: %v", err)
	}
	if !got.UTC().Truncate(time.Microsecond).Equal(second) {
		t.Errorf("after second record: expected %v, got %v", second, got)
	}
}

func TestLastPolledInstanceIsolation(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	tracker := NewSQLitePollTracker(db)
	ctx := context.Background()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	instRepo := instance.NewSQLiteRepository(db, encKey)

	inst1 := instance.Instance{
		Name:      "poll-isolation-1",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://sonarr1:8989",
		APIKey:    "key-1",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst1); err != nil {
		t.Fatalf("creating instance 1: %v", err)
	}

	inst2 := instance.Instance{
		Name:      "poll-isolation-2",
		AppType:   instance.AppTypeWhisparr,
		BaseURL:   "http://whisparr1:6969",
		APIKey:    "key-2",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst2); err != nil {
		t.Fatalf("creating instance 2: %v", err)
	}

	polledAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := tracker.RecordPoll(ctx, inst1.ID, polledAt); err != nil {
		t.Fatalf("RecordPoll for instance 1: %v", err)
	}

	got, err := tracker.LastPolled(ctx, inst2.ID)
	if err != nil {
		t.Fatalf("LastPolled for instance 2: %v", err)
	}
	if !got.IsZero() {
		t.Errorf(
			"expected zero time for unpolled instance 2, got %v", got)
	}

	got, err = tracker.LastPolled(ctx, inst1.ID)
	if err != nil {
		t.Fatalf("LastPolled for instance 1: %v", err)
	}
	if !got.UTC().Truncate(time.Microsecond).Equal(polledAt) {
		t.Errorf("instance 1: expected %v, got %v", polledAt, got)
	}
}
