package instance

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"uuid"

	"github.com/refringe/huntarr2/internal/database/testdb"
)

// randomKey generates a cryptographically random 32-byte AES-256-GCM key, failing the test on error.
func randomKey(t *testing.T) []byte {
	t.Helper()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating random encryption key: %v", err)
	}
	return key
}

// newTestInstance returns an Instance populated with valid fields; each call uses a unique name.
func newTestInstance(name string, appType AppType) *Instance {
	return &Instance{
		Name:      name,
		AppType:   appType,
		BaseURL:   "http://localhost:8989",
		APIKey:    "test-api-key-secret-value",
		TimeoutMs: 30000,
	}
}

func TestSQLiteRepository_CreateAndGet(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	inst := newTestInstance("Sonarr Production", AppTypeSonarr)

	if err := repo.Create(ctx, inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inst.ID == uuid.Nil() {
		t.Fatal("expected non-nil UUID after Create")
	}
	if inst.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if inst.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}

	got, err := repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != inst.ID {
		t.Errorf("ID = %s, want %s", got.ID, inst.ID)
	}
	if got.Name != "Sonarr Production" {
		t.Errorf("Name = %q, want %q", got.Name, "Sonarr Production")
	}
	if got.AppType != AppTypeSonarr {
		t.Errorf("AppType = %q, want %q", got.AppType, AppTypeSonarr)
	}
	if got.BaseURL != "http://localhost:8989" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://localhost:8989")
	}
	if got.APIKey != "test-api-key-secret-value" {
		t.Errorf("APIKey = %q, want plaintext after decryption", got.APIKey)
	}
	if got.TimeoutMs != 30000 {
		t.Errorf("TimeoutMs = %d, want 30000", got.TimeoutMs)
	}
}

func TestSQLiteRepository_CreateGeneratesUUID(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	a := newTestInstance("Instance A", AppTypeSonarr)
	b := newTestInstance("Instance B", AppTypeRadarr)

	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("Create b: %v", err)
	}

	if a.ID == uuid.Nil() || b.ID == uuid.Nil() {
		t.Fatal("expected non-nil UUIDs")
	}
	if a.ID == b.ID {
		t.Error("expected distinct UUIDs for different instances")
	}
}

func TestSQLiteRepository_GetNotFound(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))

	_, err := repo.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteRepository_List(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 instances, got %d", len(got))
	}

	names := []string{"Charlie", "Alpha", "Bravo"}
	for _, name := range names {
		inst := newTestInstance(name, AppTypeSonarr)
		if err := repo.Create(ctx, inst); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	got, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(got))
	}

	want := []string{"Alpha", "Bravo", "Charlie"}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("List[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestSQLiteRepository_Update(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	inst := newTestInstance("Before Update", AppTypeRadarr)
	if err := repo.Create(ctx, inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	inst.Name = "After Update"
	inst.BaseURL = "http://localhost:7878"
	inst.APIKey = "new-api-key-value"
	inst.TimeoutMs = 60000

	if err := repo.Update(ctx, inst); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if got.Name != "After Update" {
		t.Errorf("Name = %q, want %q", got.Name, "After Update")
	}
	if got.BaseURL != "http://localhost:7878" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://localhost:7878")
	}
	if got.APIKey != "new-api-key-value" {
		t.Errorf("APIKey = %q, want %q", got.APIKey, "new-api-key-value")
	}
	if got.TimeoutMs != 60000 {
		t.Errorf("TimeoutMs = %d, want 60000", got.TimeoutMs)
	}
}

func TestSQLiteRepository_UpdateNotFound(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))

	inst := &Instance{
		ID:        uuid.New(),
		Name:      "Ghost",
		AppType:   AppTypeRadarr,
		BaseURL:   "http://localhost:7878",
		APIKey:    "some-key",
		TimeoutMs: 30000,
	}

	err := repo.Update(context.Background(), inst)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	inst := newTestInstance("Doomed Instance", AppTypeLidarr)
	if err := repo.Create(ctx, inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, inst.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.Get(ctx, inst.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after Delete, got %v", err)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty list after Delete, got %d", len(all))
	}
}

func TestSQLiteRepository_DeleteNotFound(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))

	err := repo.Delete(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteRepository_APIKeyEncryptedAtRest(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	key := randomKey(t)
	repo := NewSQLiteRepository(db, key)
	ctx := context.Background()

	plaintext := "super-secret-api-key-12345"
	inst := newTestInstance("Encrypted Key Test", AppTypeWhisparrV2)
	inst.APIKey = plaintext

	if err := repo.Create(ctx, inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var rawValue string
	err := db.QueryRowContext(ctx,
		`SELECT api_key_enc FROM instances WHERE id = ?`, inst.ID.String(),
	).Scan(&rawValue)
	if err != nil {
		t.Fatalf("querying raw api_key_enc: %v", err)
	}

	if rawValue == plaintext {
		t.Fatal("api_key_enc contains plaintext; expected encrypted value")
	}
	if rawValue == "" {
		t.Fatal("api_key_enc is empty; expected encrypted value")
	}

	got, err := repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKey != plaintext {
		t.Errorf("decrypted APIKey = %q, want %q", got.APIKey, plaintext)
	}
}

func TestSQLiteRepository_CRUDLifecycle(t *testing.T) {
	t.Parallel()

	db := testdb.New(t)
	repo := NewSQLiteRepository(db, randomKey(t))
	ctx := context.Background()

	inst := newTestInstance("Lifecycle Test", AppTypeSonarr)
	if err := repo.Create(ctx, inst); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inst.ID == uuid.Nil() {
		t.Fatal("expected non-nil UUID after Create")
	}

	got, err := repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Lifecycle Test" {
		t.Errorf("Name = %q, want %q", got.Name, "Lifecycle Test")
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 instance in List, got %d", len(all))
	}

	inst.Name = "Lifecycle Updated"
	inst.APIKey = "updated-key"
	if err := repo.Update(ctx, inst); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err = repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if got.Name != "Lifecycle Updated" {
		t.Errorf("Name after Update = %q, want %q", got.Name, "Lifecycle Updated")
	}
	if got.APIKey != "updated-key" {
		t.Errorf("APIKey after Update = %q, want %q", got.APIKey, "updated-key")
	}

	if err := repo.Delete(ctx, inst.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.Get(ctx, inst.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after Delete, got %v", err)
	}

	all, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty list after Delete, got %d", len(all))
	}
}

func TestSQLiteRepository_UndecryptableKeyReadsAsEmpty(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := t.Context()

	original := NewSQLiteRepository(db, randomKey(t))
	readable := newTestInstance("rekeyed", AppTypeSonarr)
	if err := original.Create(ctx, readable); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rekeyed := NewSQLiteRepository(db, randomKey(t))

	listed, err := rekeyed.List(ctx)
	if err != nil {
		t.Fatalf("List with a different encryption key: %v", err)
	}
	if len(listed) != 1 || listed[0].APIKey != "" {
		t.Fatalf("List = %+v, want the one instance with an empty APIKey", listed)
	}

	got, err := rekeyed.Get(ctx, readable.ID)
	if err != nil {
		t.Fatalf("Get with a different encryption key: %v", err)
	}
	if got.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", got.APIKey)
	}

	got.APIKey = "re-entered-key"
	if err := rekeyed.Update(ctx, &got); err != nil {
		t.Fatalf("Update with a re-entered key: %v", err)
	}
	got, err = rekeyed.Get(ctx, readable.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.APIKey != "re-entered-key" {
		t.Errorf("APIKey = %q, want the re-entered key", got.APIKey)
	}
}
