package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	global   []Setting
	instance map[uuid.UUID][]Setting
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{instance: make(map[uuid.UUID][]Setting)}
}

func (f *fakeRepository) ListGlobal(_ context.Context) ([]Setting, error) {
	return f.global, nil
}

func (f *fakeRepository) ListByInstance(_ context.Context, id uuid.UUID) ([]Setting, error) {
	return f.instance[id], nil
}

func (f *fakeRepository) Upsert(_ context.Context, s *Setting) error {
	now := time.Now()
	if s.InstanceID == nil {
		for i, existing := range f.global {
			if existing.Key == s.Key {
				f.global[i].Value = s.Value
				f.global[i].UpdatedAt = now
				s.ID = existing.ID
				s.UpdatedAt = now
				return nil
			}
		}
		s.ID = uuid.New()
		s.UpdatedAt = now
		f.global = append(f.global, *s)
		return nil
	}
	id := *s.InstanceID
	for i, existing := range f.instance[id] {
		if existing.Key == s.Key {
			f.instance[id][i].Value = s.Value
			f.instance[id][i].UpdatedAt = now
			s.ID = existing.ID
			s.UpdatedAt = now
			return nil
		}
	}
	s.ID = uuid.New()
	s.UpdatedAt = now
	f.instance[id] = append(f.instance[id], *s)
	return nil
}

func (f *fakeRepository) UpsertBatch(_ context.Context, settings []Setting) error {
	for _, s := range settings {
		if err := f.Upsert(context.Background(), &s); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeRepository) Delete(_ context.Context, instanceID *uuid.UUID, key string) error {
	if instanceID == nil {
		for i, s := range f.global {
			if s.Key == key {
				f.global = append(f.global[:i], f.global[i+1:]...)
				return nil
			}
		}
		return nil
	}
	id := *instanceID
	for i, s := range f.instance[id] {
		if s.Key == key {
			f.instance[id] = append(f.instance[id][:i], f.instance[id][i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeRepository) DeleteBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error {
	for _, key := range keys {
		if err := f.Delete(ctx, instanceID, key); err != nil {
			return err
		}
	}
	return nil
}

func TestResolveDefaults(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepository())
	r, err := svc.ResolveGlobal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := Defaults()
	if r.BatchSize != d.BatchSize {
		t.Errorf("BatchSize = %d, want %d", r.BatchSize, d.BatchSize)
	}
	if r.CooldownPeriod != d.CooldownPeriod {
		t.Errorf("CooldownPeriod = %v, want %v", r.CooldownPeriod, d.CooldownPeriod)
	}
	if r.SearchInterval != d.SearchInterval {
		t.Errorf("SearchInterval = %v, want %v", r.SearchInterval, d.SearchInterval)
	}
	if r.SearchLimit != d.SearchLimit {
		t.Errorf("SearchLimit = %d, want %d", r.SearchLimit, d.SearchLimit)
	}
	if r.Enabled != d.Enabled {
		t.Errorf("Enabled = %v, want %v", r.Enabled, d.Enabled)
	}
}

func TestResolvePrecedence(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	repo.global = []Setting{
		{Key: KeyBatchSize, Value: "20"},
		{Key: KeyCooldownPeriod, Value: "12h"},
	}

	instID := uuid.New()
	repo.instance[instID] = []Setting{
		{Key: KeyBatchSize, Value: "5"},
	}

	svc := NewService(repo)

	t.Run("global overrides defaults", func(t *testing.T) {
		r, err := svc.ResolveGlobal(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.BatchSize != 20 {
			t.Errorf("BatchSize = %d, want 20", r.BatchSize)
		}
		if r.CooldownPeriod != 12*time.Hour {
			t.Errorf("CooldownPeriod = %v, want 12h", r.CooldownPeriod)
		}
	})

	t.Run("per-instance overrides global", func(t *testing.T) {
		r, err := svc.Resolve(context.Background(), instID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.BatchSize != 5 {
			t.Errorf("BatchSize = %d, want 5", r.BatchSize)
		}
		if r.CooldownPeriod != 12*time.Hour {
			t.Errorf("CooldownPeriod = %v, want 12h (from global)", r.CooldownPeriod)
		}
	})
}

func TestSetInvalidKey(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepository())
	err := svc.Set(context.Background(), nil, "nonexistent_key", "42")
	if !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
}

func TestSetInvalidValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key   string
		value string
	}{
		{KeyBatchSize, "not_a_number"},
		{KeyBatchSize, "0"},
		{KeyBatchSize, "-5"},
		{KeyCooldownPeriod, "not_a_duration"},
		{KeySearchInterval, "bad"},
		{KeySearchLimit, "abc"},
		{KeySearchLimit, "0"},
		{KeySearchLimit, "-1"},
		{KeyEnabled, "maybe"},
		{KeySearchWindowStart, "25:00"},
		{KeySearchWindowEnd, "12:99"},
	}

	svc := NewService(newFakeRepository())
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			err := svc.Set(context.Background(), nil, tt.key, tt.value)
			if !errors.Is(err, ErrInvalidValue) {
				t.Errorf("err = %v, want ErrInvalidValue", err)
			}
		})
	}
}

func TestSetValidValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key   string
		value string
	}{
		{KeyBatchSize, "50"},
		{KeyCooldownPeriod, "6h"},
		{KeySearchInterval, "30m"},
		{KeySearchLimit, "200"},
		{KeyEnabled, "false"},
		{KeySearchWindowStart, "01:00"},
		{KeySearchWindowEnd, "06:30"},
		{KeySearchWindowStart, ""},
	}

	svc := NewService(newFakeRepository())
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			err := svc.Set(context.Background(), nil, tt.key, tt.value)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	repo.global = []Setting{
		{Key: KeyBatchSize, Value: "20"},
	}

	svc := NewService(repo)
	if err := svc.Remove(context.Background(), nil, KeyBatchSize); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r, err := svc.ResolveGlobal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.BatchSize != Defaults().BatchSize {
		t.Errorf("BatchSize = %d, want default %d", r.BatchSize, Defaults().BatchSize)
	}
}

func TestRemoveInvalidKey(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepository())
	err := svc.Remove(context.Background(), nil, "bad_key")
	if !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
}

func TestValidKey(t *testing.T) {
	t.Parallel()
	if !ValidKey(KeyBatchSize) {
		t.Error("ValidKey(KeyBatchSize) = false, want true")
	}
	if ValidKey("nonexistent") {
		t.Error("ValidKey(\"nonexistent\") = true, want false")
	}
}

func TestParseHHMM(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"midnight", "00:00", 0, false},
		{"noon", "12:00", 720, false},
		{"end of day", "23:59", 1439, false},
		{"morning", "09:30", 570, false},
		{"single digit hour", "9:30", 0, true},
		{"hour too high", "25:00", 0, true},
		{"minute too high", "12:99", 0, true},
		{"negative hour", "-1:00", 0, true},
		{"empty string", "", 0, true},
		{"no colon", "1200", 0, true},
		{"too long", "12:000", 0, true},
		{"letters", "ab:cd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHHMM(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHHMM(%q) error = %v, wantErr %v",
					tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseHHMM(%q) = %d, want %d",
					tt.input, got, tt.want)
			}
		})
	}
}
