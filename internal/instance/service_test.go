package instance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRepository implements Repository for unit tests using in-memory storage.
type fakeRepository struct {
	instances map[uuid.UUID]Instance
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{instances: make(map[uuid.UUID]Instance)}
}

func (f *fakeRepository) List(_ context.Context) ([]Instance, error) {
	out := make([]Instance, 0, len(f.instances))
	for _, inst := range f.instances {
		out = append(out, inst)
	}
	return out, nil
}

func (f *fakeRepository) ListByType(_ context.Context, appType AppType) ([]Instance, error) {
	var out []Instance
	for _, inst := range f.instances {
		if inst.AppType == appType {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (f *fakeRepository) Get(_ context.Context, id uuid.UUID) (Instance, error) {
	inst, ok := f.instances[id]
	if !ok {
		return Instance{}, ErrNotFound
	}
	return inst, nil
}

func (f *fakeRepository) Create(_ context.Context, inst *Instance) error {
	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}
	f.instances[inst.ID] = *inst
	return nil
}

func (f *fakeRepository) Update(_ context.Context, inst *Instance) error {
	if _, ok := f.instances[inst.ID]; !ok {
		return ErrNotFound
	}
	f.instances[inst.ID] = *inst
	return nil
}

func (f *fakeRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.instances[id]; !ok {
		return ErrNotFound
	}
	delete(f.instances, id)
	return nil
}

func validInstance() *Instance {
	return &Instance{
		Name:      "My Sonarr",
		AppType:   AppTypeSonarr,
		BaseURL:   "http://localhost:8989",
		APIKey:    "abc123",
		TimeoutMs: 30000,
	}
}

func TestCreateValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(inst *Instance)
		wantErr string
		field   string
	}{
		{
			name:    "empty name",
			modify:  func(inst *Instance) { inst.Name = "" },
			wantErr: "must not be empty",
			field:   "name",
		},
		{
			name:    "whitespace name",
			modify:  func(inst *Instance) { inst.Name = "   " },
			wantErr: "must not be empty",
			field:   "name",
		},
		{
			name:    "invalid app type",
			modify:  func(inst *Instance) { inst.AppType = "netflix" },
			wantErr: "not a recognised application type",
			field:   "app_type",
		},
		{
			name:    "invalid base URL",
			modify:  func(inst *Instance) { inst.BaseURL = "not-a-url" },
			wantErr: "must be a valid HTTP or HTTPS URL",
			field:   "base_url",
		},
		{
			name:    "ftp URL rejected",
			modify:  func(inst *Instance) { inst.BaseURL = "ftp://example.com" },
			wantErr: "must be a valid HTTP or HTTPS URL",
			field:   "base_url",
		},
		{
			name:    "empty API key",
			modify:  func(inst *Instance) { inst.APIKey = "" },
			wantErr: "must not be empty",
			field:   "api_key",
		},
		{
			name:    "negative timeout",
			modify:  func(inst *Instance) { inst.TimeoutMs = -1 },
			wantErr: "must be between 0 and",
			field:   "timeout_ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newFakeRepository())
			inst := validInstance()
			tt.modify(inst)

			err := svc.Create(context.Background(), inst)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}

			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Field != tt.field {
				t.Errorf("field = %q, want %q", ve.Field, tt.field)
			}
			if !errors.Is(err, ErrValidation) {
				t.Error("error does not wrap ErrValidation")
			}
		})
	}
}

func TestCreateSuccess(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	inst := validInstance()

	if err := svc.Create(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.ID == uuid.Nil {
		t.Error("expected ID to be set after create")
	}
	if len(repo.instances) != 1 {
		t.Errorf("repository has %d instances, want 1", len(repo.instances))
	}
}

func TestCreateDefaultsTimeout(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeRepository())
	inst := validInstance()
	inst.TimeoutMs = 0

	if err := svc.Create(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.TimeoutMs != 15000 {
		t.Errorf("TimeoutMs = %d, want 15000", inst.TimeoutMs)
	}
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeRepository())

	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetSuccess(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	inst := validInstance()

	if err := svc.Create(context.Background(), inst); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.Get(context.Background(), inst.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != inst.Name {
		t.Errorf("name = %q, want %q", got.Name, inst.Name)
	}
}

func TestUpdateAppliesChanges(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	inst := validInstance()

	if err := svc.Create(context.Background(), inst); err != nil {
		t.Fatalf("create: %v", err)
	}

	update := &Instance{
		Name:      "Renamed Sonarr",
		BaseURL:   "http://localhost:7878",
		APIKey:    "newkey456",
		TimeoutMs: 15000,
	}

	got, err := svc.Update(context.Background(), inst.ID, update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "Renamed Sonarr" {
		t.Errorf("name = %q, want %q", got.Name, "Renamed Sonarr")
	}
	if got.BaseURL != "http://localhost:7878" {
		t.Errorf("base_url = %q, want %q", got.BaseURL, "http://localhost:7878")
	}
	if got.AppType != AppTypeSonarr {
		t.Error("app type should not change on update")
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeRepository())
	update := &Instance{
		Name:    "Ghost",
		BaseURL: "http://localhost:9696",
		APIKey:  "key",
	}

	_, err := svc.Update(context.Background(), uuid.New(), update)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteSuccess(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	inst := validInstance()

	if err := svc.Create(context.Background(), inst); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Delete(context.Background(), inst.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.instances) != 0 {
		t.Error("expected repository to be empty after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeRepository())

	err := svc.Delete(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListReturnsAll(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)

	for _, name := range []string{"Sonarr 1", "Radarr 1"} {
		inst := validInstance()
		inst.Name = name
		if err := svc.Create(context.Background(), inst); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestAppTypeValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		appType AppType
		want    bool
	}{
		{AppTypeSonarr, true},
		{AppTypeRadarr, true},
		{AppTypeLidarr, true},
		{AppTypeWhisparr, true},
		{"prowlarr", false},
		{"netflix", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.appType), func(t *testing.T) {
			if got := tt.appType.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
