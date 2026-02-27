package activity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	entries []Entry
}

func (f *fakeRepository) Create(_ context.Context, entry *Entry) error {
	entry.ID = uuid.New()
	entry.CreatedAt = time.Now()
	f.entries = append(f.entries, *entry)
	return nil
}

func (f *fakeRepository) List(_ context.Context, params ListParams) ([]Entry, error) {
	var out []Entry
	for _, e := range f.entries {
		if params.Level != "" && e.Level != params.Level {
			continue
		}
		if params.InstanceID != nil && (e.InstanceID == nil || *e.InstanceID != *params.InstanceID) {
			continue
		}
		if params.Action != "" && e.Action != params.Action {
			continue
		}
		if params.Search != "" && !strings.Contains(
			strings.ToLower(e.Message), strings.ToLower(params.Search)) {
			continue
		}
		if params.Since != nil && e.CreatedAt.Before(*params.Since) {
			continue
		}
		if params.Until != nil && !e.CreatedAt.Before(*params.Until) {
			continue
		}
		out = append(out, e)
	}

	if params.Offset > 0 && params.Offset < len(out) {
		out = out[params.Offset:]
	} else if params.Offset >= len(out) {
		out = nil
	}
	if params.Limit > 0 && params.Limit < len(out) {
		out = out[:params.Limit]
	}
	return out, nil
}

func (f *fakeRepository) Count(_ context.Context, params ListParams) (int, error) {
	entries, _ := f.List(context.Background(), ListParams{
		Level:      params.Level,
		InstanceID: params.InstanceID,
		Action:     params.Action,
		Search:     params.Search,
		Since:      params.Since,
		Until:      params.Until,
	})
	return len(entries), nil
}

func (f *fakeRepository) DeleteBefore(_ context.Context, before time.Time) (int64, error) {
	var kept []Entry
	var deleted int64
	for _, e := range f.entries {
		if e.CreatedAt.Before(before) {
			deleted++
		} else {
			kept = append(kept, e)
		}
	}
	f.entries = kept
	return deleted, nil
}

func (f *fakeRepository) Stats(_ context.Context, since *time.Time) ([]ActionStats, error) {
	counts := make(map[string]int)
	for _, e := range f.entries {
		if since != nil && e.CreatedAt.Before(*since) {
			continue
		}
		var instKey string
		if e.InstanceID != nil {
			instKey = e.InstanceID.String()
		}
		key := instKey + "|" + string(e.Action)
		counts[key]++
	}

	var results []ActionStats
	for key, count := range counts {
		parts := strings.SplitN(key, "|", 2)
		var instID *uuid.UUID
		if parts[0] != "" {
			id, _ := uuid.Parse(parts[0])
			instID = &id
		}
		results = append(results, ActionStats{
			InstanceID: instID,
			Action:     Action(parts[1]),
			Count:      count,
		})
	}
	return results, nil
}

func TestLogValidEntry(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	entry := &Entry{
		Level:   LevelInfo,
		Action:  ActionSearchCycle,
		Message: "searched 5 items",
	}
	if err := svc.Log(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("len = %d, want 1", len(repo.entries))
	}
	if entry.ID == uuid.Nil {
		t.Error("entry ID should be assigned")
	}
}

func TestLogInvalidLevel(t *testing.T) {
	t.Parallel()
	svc := NewService(&fakeRepository{})

	entry := &Entry{
		Level:   Level("critical"),
		Action:  ActionSearchCycle,
		Message: "bad level",
	}
	if err := svc.Log(context.Background(), entry); !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("err = %v, want ErrInvalidLevel", err)
	}
}

func TestLogInvalidAction(t *testing.T) {
	t.Parallel()
	svc := NewService(&fakeRepository{})

	entry := &Entry{
		Level:   LevelInfo,
		Action:  Action("nonexistent_action"),
		Message: "bad action",
	}
	if err := svc.Log(context.Background(), entry); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("err = %v, want ErrInvalidAction", err)
	}
}

func TestListDefaultLimit(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	for range 60 {
		repo.entries = append(repo.entries, Entry{
			ID:    uuid.New(),
			Level: LevelInfo,
		})
	}

	entries, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 50 {
		t.Errorf("len = %d, want 50 (default limit)", len(entries))
	}
}

func TestListMaxLimit(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	for range 600 {
		repo.entries = append(repo.entries, Entry{
			ID:    uuid.New(),
			Level: LevelInfo,
		})
	}

	entries, err := svc.List(context.Background(), ListParams{Limit: 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 500 {
		t.Errorf("len = %d, want 500 (max limit)", len(entries))
	}
}

func TestCount(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	instID := uuid.New()
	repo.entries = []Entry{
		{ID: uuid.New(), Level: LevelInfo, InstanceID: &instID},
		{ID: uuid.New(), Level: LevelError, InstanceID: &instID},
		{ID: uuid.New(), Level: LevelInfo},
	}

	count, err := svc.Count(context.Background(), ListParams{Level: LevelInfo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestListActionFilter(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	repo.entries = []Entry{
		{ID: uuid.New(), Level: LevelInfo, Action: ActionSearchCycle},
		{ID: uuid.New(), Level: LevelInfo, Action: ActionSearchSkip},
		{ID: uuid.New(), Level: LevelInfo, Action: ActionSearchCycle},
	}

	entries, err := svc.List(context.Background(), ListParams{
		Action: ActionSearchCycle,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}
}

func TestListSearchFilter(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	repo.entries = []Entry{
		{ID: uuid.New(), Level: LevelInfo, Message: "searched 5 items for Sonarr"},
		{ID: uuid.New(), Level: LevelInfo, Message: "all items in cooldown"},
		{ID: uuid.New(), Level: LevelInfo, Message: "searched 3 items for Radarr"},
	}

	entries, err := svc.List(context.Background(), ListParams{
		Search: "searched",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}
}

func TestListSinceFilter(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	repo.entries = []Entry{
		{ID: uuid.New(), Level: LevelInfo, CreatedAt: old},
		{ID: uuid.New(), Level: LevelInfo, CreatedAt: recent},
	}

	entries, err := svc.List(context.Background(), ListParams{
		Since: &cutoff,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("len = %d, want 1", len(entries))
	}
}

func TestStatsAggregation(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	instID := uuid.New()
	now := time.Now()
	repo.entries = []Entry{
		{ID: uuid.New(), InstanceID: &instID, Action: ActionSearchCycle, CreatedAt: now},
		{ID: uuid.New(), InstanceID: &instID, Action: ActionSearchCycle, CreatedAt: now},
		{ID: uuid.New(), InstanceID: &instID, Action: ActionSearchSkip, CreatedAt: now},
		{ID: uuid.New(), Action: ActionHealthCheck, CreatedAt: now},
	}

	results, err := svc.Stats(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actionCounts := make(map[Action]int)
	for _, r := range results {
		actionCounts[r.Action] += r.Count
	}
	if actionCounts[ActionSearchCycle] != 2 {
		t.Errorf("search_cycle count = %d, want 2", actionCounts[ActionSearchCycle])
	}
	if actionCounts[ActionSearchSkip] != 1 {
		t.Errorf("search_skip count = %d, want 1", actionCounts[ActionSearchSkip])
	}
	if actionCounts[ActionHealthCheck] != 1 {
		t.Errorf("health_check count = %d, want 1", actionCounts[ActionHealthCheck])
	}
}

func TestStatsSinceFilter(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{}
	svc := NewService(repo)

	instID := uuid.New()
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	repo.entries = []Entry{
		{ID: uuid.New(), InstanceID: &instID, Action: ActionSearchCycle, CreatedAt: old},
		{ID: uuid.New(), InstanceID: &instID, Action: ActionSearchCycle, CreatedAt: now},
	}

	results, err := svc.Stats(context.Background(), &cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	total := 0
	for _, r := range results {
		total += r.Count
	}
	if total != 1 {
		t.Errorf("total count = %d, want 1", total)
	}
}
