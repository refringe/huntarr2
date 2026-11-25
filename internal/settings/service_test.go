package settings

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"
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

func (f *fakeRepository) upsert(s Setting) {
	s.UpdatedAt = time.Now()
	rows := &f.global
	if s.InstanceID != nil {
		layer := f.instance[*s.InstanceID]
		rows = &layer
		defer func() { f.instance[*s.InstanceID] = layer }()
	}
	for i, existing := range *rows {
		if existing.Key == s.Key {
			s.ID = existing.ID
			(*rows)[i] = s
			return
		}
	}
	s.ID = uuid.New()
	*rows = append(*rows, s)
}

func (f *fakeRepository) UpsertBatch(_ context.Context, settings []Setting) error {
	for _, s := range settings {
		f.upsert(s)
	}
	return nil
}

func (f *fakeRepository) DeleteBatch(_ context.Context, instanceID *uuid.UUID, keys []string) error {
	for _, key := range keys {
		if instanceID == nil {
			f.global = slices.DeleteFunc(f.global, func(s Setting) bool { return s.Key == key })
			continue
		}
		f.instance[*instanceID] = slices.DeleteFunc(f.instance[*instanceID], func(s Setting) bool { return s.Key == key })
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
	if r.SearchMissing != d.SearchMissing {
		t.Errorf("SearchMissing = %v, want %v", r.SearchMissing, d.SearchMissing)
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

func TestResolveOutOfRangeCooldownPeriod(t *testing.T) {
	t.Parallel()

	t.Run("stored oversized value is clamped to the maximum", func(t *testing.T) {
		repo := newFakeRepository()
		repo.global = []Setting{{Key: KeyCooldownPeriod, Value: "876000h"}}
		r, err := NewService(repo).ResolveGlobal(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.CooldownPeriod != MaxCooldownPeriod {
			t.Errorf("CooldownPeriod = %v, want %v", r.CooldownPeriod, MaxCooldownPeriod)
		}
	})

	t.Run("stored non-positive value falls back to the default", func(t *testing.T) {
		repo := newFakeRepository()
		repo.global = []Setting{{Key: KeyCooldownPeriod, Value: "-24h"}}
		r, err := NewService(repo).ResolveGlobal(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.CooldownPeriod != Defaults().CooldownPeriod {
			t.Errorf("CooldownPeriod = %v, want %v", r.CooldownPeriod, Defaults().CooldownPeriod)
		}
	})
}

func TestResolveOutOfRangeSearchInterval(t *testing.T) {
	t.Parallel()

	t.Run("stored oversized value is clamped to the maximum", func(t *testing.T) {
		repo := newFakeRepository()
		repo.global = []Setting{{Key: KeySearchInterval, Value: "8760h"}}
		r, err := NewService(repo).ResolveGlobal(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.SearchInterval != MaxSearchInterval {
			t.Errorf("SearchInterval = %v, want %v", r.SearchInterval, MaxSearchInterval)
		}
	})

	t.Run("stored non-positive value falls back to the default", func(t *testing.T) {
		repo := newFakeRepository()
		repo.global = []Setting{{Key: KeySearchInterval, Value: "-6h"}}
		r, err := NewService(repo).ResolveGlobal(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.SearchInterval != Defaults().SearchInterval {
			t.Errorf("SearchInterval = %v, want %v", r.SearchInterval, Defaults().SearchInterval)
		}
	})
}

func TestSetInvalidKey(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepository())
	err := svc.SetBatch(context.Background(), nil, []SettingEntry{{Key: "nonexistent_key", Value: "42"}})
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
		{KeyCooldownPeriod, "0s"},
		{KeyCooldownPeriod, "-24h"},
		{KeyCooldownPeriod, "2161h"},
		{KeySearchInterval, "bad"},
		{KeySearchInterval, "0s"},
		{KeySearchInterval, "-6h"},
		{KeySearchInterval, "169h"},
		{KeySearchLimit, "abc"},
		{KeySearchLimit, "0"},
		{KeySearchLimit, "-1"},
		{KeyEnabled, "maybe"},
		{KeySearchMissing, "maybe"},
		{KeySearchWindowStart, "25:00"},
		{KeySearchWindowEnd, "12:99"},
	}

	svc := NewService(newFakeRepository())
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			err := svc.SetBatch(context.Background(), nil, []SettingEntry{{Key: tt.key, Value: tt.value}})
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
		{KeyCooldownPeriod, "2160h"},
		{KeySearchInterval, "30m"},
		{KeySearchInterval, "168h"},
		{KeySearchLimit, "200"},
		{KeyEnabled, "false"},
		{KeySearchMissing, "true"},
		{KeySearchMissing, "false"},
		{KeySearchWindowStart, "01:00"},
		{KeySearchWindowEnd, "06:30"},
		{KeySearchWindowStart, ""},
	}

	svc := NewService(newFakeRepository())
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			err := svc.SetBatch(context.Background(), nil, []SettingEntry{{Key: tt.key, Value: tt.value}})
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
	if err := svc.RemoveBatch(context.Background(), nil, []string{KeyBatchSize}); err != nil {
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
	err := svc.RemoveBatch(context.Background(), nil, []string{"bad_key"})
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
