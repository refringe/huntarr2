package settings

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/database/testdb"
	"github.com/refringe/huntarr2/internal/instance"
)

// randomEncryptionKey generates a cryptographically random 32-byte key
// suitable for AES-256-GCM encryption.
func randomEncryptionKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}
	return key
}

func TestSQLiteRepository(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	encryptionKey := randomEncryptionKey(t)

	instRepo := instance.NewSQLiteRepository(db, encryptionKey)
	inst := instance.Instance{
		Name:      "Test",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://test:8989",
		APIKey:    "testkey123",
		TimeoutMs: 15000,
	}
	if err := instRepo.Create(ctx, &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	repo := NewSQLiteRepository(db)

	t.Run("GlobalUpsertAndList", func(t *testing.T) {
		s := &Setting{Key: KeyBatchSize, Value: "10"}
		if err := repo.Upsert(ctx, s); err != nil {
			t.Fatalf("upserting global setting: %v", err)
		}

		globals, err := repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings: %v", err)
		}

		found := false
		for _, g := range globals {
			if g.Key == KeyBatchSize && g.Value == "10" {
				if g.InstanceID != nil {
					t.Fatal("expected nil InstanceID for global setting")
				}
				if g.ID == uuid.Nil {
					t.Fatal("expected non-nil ID")
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("global setting %q not found in list", KeyBatchSize)
		}

		s2 := &Setting{Key: KeyBatchSize, Value: "20"}
		if err := repo.Upsert(ctx, s2); err != nil {
			t.Fatalf("upserting global setting (update): %v", err)
		}
		globals, err = repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings after update: %v", err)
		}
		for _, g := range globals {
			if g.Key == KeyBatchSize {
				if g.Value != "20" {
					t.Fatalf("expected value %q, got %q", "20", g.Value)
				}
			}
		}
	})

	t.Run("PerInstanceUpsertAndList", func(t *testing.T) {
		instID := inst.ID
		s := &Setting{
			InstanceID: &instID,
			Key:        KeySearchLimit,
			Value:      "50",
		}
		if err := repo.Upsert(ctx, s); err != nil {
			t.Fatalf("upserting per-instance setting: %v", err)
		}

		perInst, err := repo.ListByInstance(ctx, instID)
		if err != nil {
			t.Fatalf("listing instance settings: %v", err)
		}

		found := false
		for _, p := range perInst {
			if p.Key == KeySearchLimit && p.Value == "50" {
				if p.InstanceID == nil || *p.InstanceID != instID {
					t.Fatal("expected InstanceID to match the test instance")
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("per-instance setting %q not found in list",
				KeySearchLimit)
		}
	})

	t.Run("SameKeyGlobalAndPerInstance", func(t *testing.T) {
		instID := inst.ID

		globalSetting := &Setting{Key: KeyEnabled, Value: "true"}
		if err := repo.Upsert(ctx, globalSetting); err != nil {
			t.Fatalf("upserting global setting: %v", err)
		}

		instanceSetting := &Setting{
			InstanceID: &instID,
			Key:        KeyEnabled,
			Value:      "false",
		}
		if err := repo.Upsert(ctx, instanceSetting); err != nil {
			t.Fatalf("upserting per-instance setting: %v", err)
		}

		globals, err := repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings: %v", err)
		}
		var globalValue string
		for _, g := range globals {
			if g.Key == KeyEnabled {
				globalValue = g.Value
			}
		}
		if globalValue != "true" {
			t.Fatalf("expected global %q = %q, got %q",
				KeyEnabled, "true", globalValue)
		}

		perInst, err := repo.ListByInstance(ctx, instID)
		if err != nil {
			t.Fatalf("listing instance settings: %v", err)
		}
		var instValue string
		for _, p := range perInst {
			if p.Key == KeyEnabled {
				instValue = p.Value
			}
		}
		if instValue != "false" {
			t.Fatalf("expected instance %q = %q, got %q",
				KeyEnabled, "false", instValue)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s := &Setting{Key: KeyCooldownPeriod, Value: "3600"}
		if err := repo.Upsert(ctx, s); err != nil {
			t.Fatalf("upserting setting before delete: %v", err)
		}

		if err := repo.Delete(ctx, nil, KeyCooldownPeriod); err != nil {
			t.Fatalf("deleting global setting: %v", err)
		}

		globals, err := repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings after delete: %v", err)
		}
		for _, g := range globals {
			if g.Key == KeyCooldownPeriod {
				t.Fatal("setting should have been deleted")
			}
		}

		if err := repo.Delete(ctx, nil, "nonexistent_key"); err != nil {
			t.Fatalf("deleting non-existent key: %v", err)
		}
	})

	t.Run("DeletePerInstance", func(t *testing.T) {
		instID := inst.ID
		s := &Setting{
			InstanceID: &instID,
			Key:        KeySearchInterval,
			Value:      "1800",
		}
		if err := repo.Upsert(ctx, s); err != nil {
			t.Fatalf("upserting per-instance setting: %v", err)
		}

		if err := repo.Delete(ctx, &instID, KeySearchInterval); err != nil {
			t.Fatalf("deleting per-instance setting: %v", err)
		}

		perInst, err := repo.ListByInstance(ctx, instID)
		if err != nil {
			t.Fatalf("listing instance settings after delete: %v", err)
		}
		for _, p := range perInst {
			if p.Key == KeySearchInterval {
				t.Fatal("per-instance setting should have been deleted")
			}
		}
	})

	t.Run("UpsertBatch", func(t *testing.T) {
		instID := inst.ID
		batch := []Setting{
			{Key: KeySearchWindowStart, Value: "02:00"},
			{Key: KeySearchWindowEnd, Value: "06:00"},
			{InstanceID: &instID, Key: KeySearchWindowStart, Value: "03:00"},
		}
		if err := repo.UpsertBatch(ctx, batch); err != nil {
			t.Fatalf("upserting batch: %v", err)
		}

		globals, err := repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings: %v", err)
		}
		globalMap := make(map[string]string, len(globals))
		for _, g := range globals {
			globalMap[g.Key] = g.Value
		}
		if v, ok := globalMap[KeySearchWindowStart]; !ok || v != "02:00" {
			t.Fatalf("expected global %q = %q, got %q (ok=%v)",
				KeySearchWindowStart, "02:00", v, ok)
		}
		if v, ok := globalMap[KeySearchWindowEnd]; !ok || v != "06:00" {
			t.Fatalf("expected global %q = %q, got %q (ok=%v)",
				KeySearchWindowEnd, "06:00", v, ok)
		}

		perInst, err := repo.ListByInstance(ctx, instID)
		if err != nil {
			t.Fatalf("listing instance settings: %v", err)
		}
		instMap := make(map[string]string, len(perInst))
		for _, p := range perInst {
			instMap[p.Key] = p.Value
		}
		if v, ok := instMap[KeySearchWindowStart]; !ok || v != "03:00" {
			t.Fatalf("expected instance %q = %q, got %q (ok=%v)",
				KeySearchWindowStart, "03:00", v, ok)
		}
	})

	t.Run("DeleteBatch", func(t *testing.T) {
		seed := []Setting{
			{Key: KeySearchWindowStart, Value: "01:00"},
			{Key: KeySearchWindowEnd, Value: "05:00"},
		}
		if err := repo.UpsertBatch(ctx, seed); err != nil {
			t.Fatalf("seeding settings for delete batch: %v", err)
		}

		keys := []string{KeySearchWindowStart, KeySearchWindowEnd}
		if err := repo.DeleteBatch(ctx, nil, keys); err != nil {
			t.Fatalf("deleting batch: %v", err)
		}

		globals, err := repo.ListGlobal(ctx)
		if err != nil {
			t.Fatalf("listing global settings after batch delete: %v", err)
		}
		for _, g := range globals {
			if g.Key == KeySearchWindowStart || g.Key == KeySearchWindowEnd {
				t.Fatalf("setting %q should have been deleted", g.Key)
			}
		}
	})

	t.Run("DeleteBatchPerInstance", func(t *testing.T) {
		instID := inst.ID
		seed := []Setting{
			{InstanceID: &instID, Key: KeySearchWindowStart, Value: "04:00"},
			{InstanceID: &instID, Key: KeySearchWindowEnd, Value: "08:00"},
		}
		if err := repo.UpsertBatch(ctx, seed); err != nil {
			t.Fatalf("seeding per-instance settings: %v", err)
		}

		keys := []string{KeySearchWindowStart, KeySearchWindowEnd}
		if err := repo.DeleteBatch(ctx, &instID, keys); err != nil {
			t.Fatalf("deleting per-instance batch: %v", err)
		}

		perInst, err := repo.ListByInstance(ctx, instID)
		if err != nil {
			t.Fatalf("listing instance settings after batch delete: %v", err)
		}
		for _, p := range perInst {
			if p.Key == KeySearchWindowStart || p.Key == KeySearchWindowEnd {
				t.Fatalf("per-instance setting %q should have been deleted",
					p.Key)
			}
		}
	})

	t.Run("ListGlobalEmpty", func(t *testing.T) {
		if _, err := repo.ListGlobal(ctx); err != nil {
			t.Fatalf("listing global settings: %v", err)
		}
	})

	t.Run("ListByInstanceNonExistent", func(t *testing.T) {
		noSettings, err := repo.ListByInstance(ctx, uuid.New())
		if err != nil {
			t.Fatalf("listing settings for non-existent instance: %v", err)
		}
		if len(noSettings) != 0 {
			t.Fatalf("expected 0 settings, got %d", len(noSettings))
		}
	})
}
