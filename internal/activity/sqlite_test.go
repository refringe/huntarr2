package activity

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/refringe/huntarr2/internal/database/testdb"
	"github.com/refringe/huntarr2/internal/instance"
)

// createTestInstance inserts a minimal instance into the database and returns
// its UUID. This satisfies the foreign key constraint on activity_log entries
// that reference an instance.
func createTestInstance(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}

	repo := instance.NewSQLiteRepository(db, encKey)
	inst := instance.Instance{
		Name:      name,
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://test:8989",
		APIKey:    "test-api-key",
		TimeoutMs: 15000,
	}
	if err := repo.Create(context.Background(), &inst); err != nil {
		t.Fatalf("creating test instance: %v", err)
	}

	return inst.ID
}

// seedEntry is a helper that inserts an activity log entry and fails the
// test if the insert returns an error.
func seedEntry(t *testing.T, repo *SQLiteRepository, e *Entry) {
	t.Helper()
	if err := repo.Create(context.Background(), e); err != nil {
		t.Fatalf("seeding entry: %v", err)
	}
}

func TestSQLiteCreateAndList(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	entry := &Entry{
		Level:   LevelInfo,
		Action:  ActionSearchCycle,
		Message: "searched 5 items",
		Details: map[string]any{"count": float64(5)},
	}
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if entry.ID == uuid.Nil {
		t.Error("expected Create to populate entry ID")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected Create to populate CreatedAt")
	}

	entries, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}

	got := entries[0]
	if got.ID != entry.ID {
		t.Errorf("ID = %v, want %v", got.ID, entry.ID)
	}
	if got.Level != LevelInfo {
		t.Errorf("Level = %q, want %q", got.Level, LevelInfo)
	}
	if got.Action != ActionSearchCycle {
		t.Errorf("Action = %q, want %q", got.Action, ActionSearchCycle)
	}
	if got.Message != "searched 5 items" {
		t.Errorf("Message = %q, want %q", got.Message, "searched 5 items")
	}
	if got.Details["count"] != float64(5) {
		t.Errorf("Details[count] = %v, want 5", got.Details["count"])
	}
}

func TestSQLiteCreateNilDetails(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	entry := &Entry{
		Level:   LevelDebug,
		Action:  ActionHealthCheck,
		Message: "health check passed",
	}
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("Create with nil Details: %v", err)
	}

	entries, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}
	if entries[0].Details != nil {
		t.Errorf("Details = %v, want nil for entry with no details",
			entries[0].Details)
	}
}

func TestSQLiteCount(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "a"})
	seedEntry(t, repo, &Entry{Level: LevelError, Action: ActionSearchCycle, Message: "b"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionUpgradeDetected, Message: "c"})

	total, err := repo.Count(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("Count (all): %v", err)
	}
	if total != 3 {
		t.Errorf("Count (all) = %d, want 3", total)
	}

	infoCount, err := repo.Count(context.Background(), ListParams{Level: LevelInfo})
	if err != nil {
		t.Fatalf("Count (info): %v", err)
	}
	if infoCount != 2 {
		t.Errorf("Count (info) = %d, want 2", infoCount)
	}
}

func TestSQLiteFilterByLevel(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "info entry"})
	seedEntry(t, repo, &Entry{Level: LevelDebug, Action: ActionHealthCheck, Message: "debug entry"})
	seedEntry(t, repo, &Entry{Level: LevelError, Action: ActionSearchCycle, Message: "error entry"})

	entries, err := repo.List(context.Background(), ListParams{Level: LevelError, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "error entry" {
		t.Errorf("Message = %q, want %q", entries[0].Message, "error entry")
	}
}

func TestSQLiteFilterByAction(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "cycle"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionUpgradeDetected, Message: "upgrade"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionDownloadDetected, Message: "download"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "cycle again"})

	entries, err := repo.List(context.Background(), ListParams{
		Action: ActionSearchCycle,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Action != ActionSearchCycle {
			t.Errorf("Action = %q, want %q", e.Action, ActionSearchCycle)
		}
	}
}

func TestSQLiteFilterByInstanceID(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	instA := createTestInstance(t, db, "filter-inst-a")
	instB := createTestInstance(t, db, "filter-inst-b")

	seedEntry(t, repo, &Entry{InstanceID: &instA, Level: LevelInfo, Action: ActionSearchCycle, Message: "inst A"})
	seedEntry(t, repo, &Entry{InstanceID: &instB, Level: LevelInfo, Action: ActionSearchCycle, Message: "inst B"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionHealthCheck, Message: "no instance"})

	entries, err := repo.List(context.Background(), ListParams{
		InstanceID: &instA,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "inst A" {
		t.Errorf("Message = %q, want %q", entries[0].Message, "inst A")
	}
}

func TestSQLiteFilterBySearch(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "searched 5 items for Sonarr"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchSkip, Message: "all items in cooldown"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "searched 3 items for Radarr"})

	entries, err := repo.List(context.Background(), ListParams{
		Search: "searched",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
}

func TestSQLiteFilterBySearchCaseInsensitive(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "Upgrade detected for Breaking Bad"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "searched items"})

	entries, err := repo.List(context.Background(), ListParams{
		Search: "upgrade",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "Upgrade detected for Breaking Bad" {
		t.Errorf("Message = %q, want match on case-insensitive search",
			entries[0].Message)
	}
}

func TestSQLiteFilterByTimeRange(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "old"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "middle"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "recent"})

	all, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	now := time.Now().UTC()
	timestamps := map[string]time.Time{
		"old":    now.Add(-72 * time.Hour),
		"middle": now.Add(-24 * time.Hour),
		"recent": now.Add(-1 * time.Hour),
	}
	for _, e := range all {
		ts := timestamps[e.Message]
		if _, err := db.ExecContext(ctx,
			"UPDATE activity_log SET created_at = ? WHERE id = ?",
			ts.Format(time.RFC3339Nano), e.ID.String()); err != nil {
			t.Fatalf("updating created_at: %v", err)
		}
	}

	since := now.Add(-48 * time.Hour)
	until := now.Add(-2 * time.Hour)
	entries, err := repo.List(ctx, ListParams{
		Since: &since,
		Until: &until,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("List with time range: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "middle" {
		t.Errorf("Message = %q, want %q", entries[0].Message, "middle")
	}
}

func TestSQLitePagination(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	for i := range 5 {
		seedEntry(t, repo, &Entry{
			Level:   LevelInfo,
			Action:  ActionSearchCycle,
			Message: string(rune('a' + i)),
		})
	}

	page1, err := repo.List(context.Background(), ListParams{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1 len = %d, want 2", len(page1))
	}

	page2, err := repo.List(context.Background(), ListParams{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2 len = %d, want 2", len(page2))
	}

	page3, err := repo.List(context.Background(), ListParams{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("List page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page 3 len = %d, want 1", len(page3))
	}

	seen := make(map[uuid.UUID]bool)
	for _, e := range append(append(page1, page2...), page3...) {
		if seen[e.ID] {
			t.Errorf("duplicate entry %v across pages", e.ID)
		}
		seen[e.ID] = true
	}
	if len(seen) != 5 {
		t.Errorf("total unique entries = %d, want 5", len(seen))
	}
}

func TestSQLiteListOrderDescending(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "first"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "second"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "third"})

	all, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	now := time.Now().UTC()
	offsets := map[string]time.Duration{
		"first":  -3 * time.Hour,
		"second": -2 * time.Hour,
		"third":  -1 * time.Hour,
	}
	for _, e := range all {
		ts := now.Add(offsets[e.Message])
		if _, err := db.ExecContext(ctx,
			"UPDATE activity_log SET created_at = ? WHERE id = ?",
			ts.Format(time.RFC3339Nano), e.ID.String()); err != nil {
			t.Fatalf("updating created_at: %v", err)
		}
	}

	entries, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[0].Message != "third" {
		t.Errorf("entries[0].Message = %q, want %q",
			entries[0].Message, "third")
	}
	if entries[1].Message != "second" {
		t.Errorf("entries[1].Message = %q, want %q",
			entries[1].Message, "second")
	}
	if entries[2].Message != "first" {
		t.Errorf("entries[2].Message = %q, want %q",
			entries[2].Message, "first")
	}
}

func TestSQLiteStats(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "a"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "b"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionUpgradeDetected, Message: "c"})
	seedEntry(t, repo, &Entry{Level: LevelError, Action: ActionDownloadDetected, Message: "d"})

	results, err := repo.Stats(context.Background(), nil)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	actionCounts := make(map[Action]int)
	for _, r := range results {
		actionCounts[r.Action] += r.Count
	}
	if actionCounts[ActionSearchCycle] != 2 {
		t.Errorf("search_cycle = %d, want 2",
			actionCounts[ActionSearchCycle])
	}
	if actionCounts[ActionUpgradeDetected] != 1 {
		t.Errorf("upgrade_detected = %d, want 1",
			actionCounts[ActionUpgradeDetected])
	}
	if actionCounts[ActionDownloadDetected] != 1 {
		t.Errorf("download_detected = %d, want 1",
			actionCounts[ActionDownloadDetected])
	}
}

func TestSQLiteStatsSinceFilter(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "old"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "recent"})

	all, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	now := time.Now().UTC()
	for _, e := range all {
		var ts time.Time
		if e.Message == "old" {
			ts = now.Add(-72 * time.Hour)
		} else {
			ts = now.Add(-1 * time.Hour)
		}
		if _, err := db.ExecContext(ctx,
			"UPDATE activity_log SET created_at = ? WHERE id = ?",
			ts.Format(time.RFC3339Nano), e.ID.String()); err != nil {
			t.Fatalf("updating created_at: %v", err)
		}
	}

	since := now.Add(-24 * time.Hour)
	results, err := repo.Stats(ctx, &since)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	total := 0
	for _, r := range results {
		total += r.Count
	}
	if total != 1 {
		t.Errorf("total stats count = %d, want 1", total)
	}
}

func TestSQLiteDeleteBefore(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "keep"})
	seedEntry(t, repo, &Entry{Level: LevelDebug, Action: ActionHealthCheck, Message: "prune-a"})
	seedEntry(t, repo, &Entry{Level: LevelError, Action: ActionSearchCycle, Message: "prune-b"})

	all, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	now := time.Now().UTC()
	for _, e := range all {
		var ts time.Time
		if e.Message == "keep" {
			ts = now.Add(-1 * time.Hour)
		} else {
			ts = now.Add(-72 * time.Hour)
		}
		if _, err := db.ExecContext(ctx,
			"UPDATE activity_log SET created_at = ? WHERE id = ?",
			ts.Format(time.RFC3339Nano), e.ID.String()); err != nil {
			t.Fatalf("updating created_at: %v", err)
		}
	}

	cutoff := now.Add(-24 * time.Hour)
	deleted, err := repo.DeleteBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	remaining, err := repo.List(ctx, ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining = %d, want 1", len(remaining))
	}
	if remaining[0].Message != "keep" {
		t.Errorf("Message = %q, want %q", remaining[0].Message, "keep")
	}
}

func TestSQLiteDeleteBeforeNoMatch(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "recent"})

	cutoff := time.Now().UTC().Add(-9999 * time.Hour)
	deleted, err := repo.DeleteBefore(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestSQLiteCombinedFilters(t *testing.T) {
	db := testdb.New(t)
	repo := NewSQLiteRepository(db)

	instID := createTestInstance(t, db, "combined-filter-inst")

	seedEntry(t, repo, &Entry{InstanceID: &instID, Level: LevelInfo, Action: ActionSearchCycle, Message: "searched Sonarr"})
	seedEntry(t, repo, &Entry{InstanceID: &instID, Level: LevelError, Action: ActionSearchCycle, Message: "searched Sonarr failed"})
	seedEntry(t, repo, &Entry{InstanceID: &instID, Level: LevelInfo, Action: ActionUpgradeDetected, Message: "upgrade found"})
	seedEntry(t, repo, &Entry{Level: LevelInfo, Action: ActionSearchCycle, Message: "searched Radarr"})

	entries, err := repo.List(context.Background(), ListParams{
		InstanceID: &instID,
		Level:      LevelInfo,
		Action:     ActionSearchCycle,
		Search:     "Sonarr",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "searched Sonarr" {
		t.Errorf("Message = %q, want %q",
			entries[0].Message, "searched Sonarr")
	}
}
