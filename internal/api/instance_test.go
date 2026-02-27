package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/instance"
)

type fakeInstanceService struct {
	instances map[uuid.UUID]instance.Instance
}

func newFakeInstanceService() *fakeInstanceService {
	return &fakeInstanceService{instances: make(map[uuid.UUID]instance.Instance)}
}

func (f *fakeInstanceService) List(_ context.Context) ([]instance.Instance, error) {
	out := make([]instance.Instance, 0, len(f.instances))
	for _, inst := range f.instances {
		out = append(out, inst)
	}
	return out, nil
}

func (f *fakeInstanceService) Get(_ context.Context, id uuid.UUID) (instance.Instance, error) {
	inst, ok := f.instances[id]
	if !ok {
		return instance.Instance{}, instance.ErrNotFound
	}
	return inst, nil
}

func (f *fakeInstanceService) Create(_ context.Context, inst *instance.Instance) error {
	inst.ID = uuid.New()
	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	f.instances[inst.ID] = *inst
	return nil
}

func (f *fakeInstanceService) Update(_ context.Context, id uuid.UUID, inst *instance.Instance) (instance.Instance, error) {
	existing, ok := f.instances[id]
	if !ok {
		return instance.Instance{}, instance.ErrNotFound
	}
	existing.Name = inst.Name
	existing.BaseURL = inst.BaseURL
	existing.APIKey = inst.APIKey
	if inst.TimeoutMs > 0 {
		existing.TimeoutMs = inst.TimeoutMs
	}
	existing.UpdatedAt = time.Now()
	f.instances[id] = existing
	return existing, nil
}

func (f *fakeInstanceService) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.instances[id]; !ok {
		return instance.ErrNotFound
	}
	delete(f.instances, id)
	return nil
}

func (f *fakeInstanceService) seed(id uuid.UUID) {
	f.instances[id] = instance.Instance{
		ID:        id,
		Name:      "Test Sonarr",
		AppType:   instance.AppTypeSonarr,
		BaseURL:   "http://sonarr:8989",
		APIKey:    "testkey",
		TimeoutMs: 30000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestHandleListInstances(t *testing.T) {
	t.Parallel()
	svc := newFakeInstanceService()
	svc.seed(uuid.New())
	handler := NewRouter(svc, nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body []instanceResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("len = %d, want 1", len(body))
	}
}

func TestHandleCreateInstanceValid(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	body := `{"name":"My Sonarr","appType":"sonarr","baseUrl":"http://sonarr:8989","apiKey":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/instances", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp instanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Name != "My Sonarr" {
		t.Errorf("name = %q, want %q", resp.Name, "My Sonarr")
	}
}

func TestHandleCreateInstanceInvalid(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodPost, "/instances", bytes.NewBufferString(`not json`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGetInstanceFound(t *testing.T) {
	t.Parallel()
	svc := newFakeInstanceService()
	id := uuid.New()
	svc.seed(id)
	handler := NewRouter(svc, nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/instances/"+id.String(), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetInstanceNotFound(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/instances/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleGetInstanceBadUUID(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/instances/not-a-uuid", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateInstance(t *testing.T) {
	t.Parallel()
	svc := newFakeInstanceService()
	id := uuid.New()
	svc.seed(id)
	handler := NewRouter(svc, nil, nil, nil, nil).Handler()

	body := `{"name":"Renamed","baseUrl":"https://sonarr.example.com","apiKey":"newkey"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/"+id.String(), bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp instanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Name != "Renamed" {
		t.Errorf("name = %q, want %q", resp.Name, "Renamed")
	}
	if resp.BaseURL != "https://sonarr.example.com" {
		t.Errorf("baseUrl = %q, want %q", resp.BaseURL, "https://sonarr.example.com")
	}
}

func TestHandleUpdateInstanceNotFound(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	body := `{"name":"Ghost","baseUrl":"http://localhost:8989","apiKey":"key"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/"+uuid.New().String(), bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteInstance(t *testing.T) {
	t.Parallel()
	svc := newFakeInstanceService()
	id := uuid.New()
	svc.seed(id)
	handler := NewRouter(svc, nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodDelete, "/instances/"+id.String(), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteInstanceNotFound(t *testing.T) {
	t.Parallel()
	handler := NewRouter(newFakeInstanceService(), nil, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodDelete, "/instances/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleTestInstanceSuccess(t *testing.T) {
	t.Parallel()
	svc := newFakeInstanceService()
	id := uuid.New()
	svc.seed(id)

	arrSvc := &fakeArrService{testErr: nil}
	handler := NewRouter(svc, arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodPost, "/instances/"+id.String()+"/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestHandleTestInstanceNotFound(t *testing.T) {
	t.Parallel()
	arrSvc := &fakeArrService{}
	handler := NewRouter(newFakeInstanceService(), arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodPost, "/instances/"+uuid.New().String()+"/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleTestInstanceAppTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		appType    instance.AppType
		baseURL    string
		testErr    error
		wantCode   int
		wantStatus string
		wantMsg    string
	}{
		{
			name:       "sonarr success",
			appType:    instance.AppTypeSonarr,
			baseURL:    "http://sonarr:8989",
			wantStatus: "ok",
		},
		{
			name:       "lidarr success",
			appType:    instance.AppTypeLidarr,
			baseURL:    "http://lidarr:8686",
			wantStatus: "ok",
		},
		{
			name:       "connection failure",
			appType:    instance.AppTypeSonarr,
			baseURL:    "http://sonarr:8989",
			testErr:    errors.New("connection refused"),
			wantCode:   http.StatusBadGateway,
			wantStatus: "failed",
			wantMsg:    "connection test failed; check the instance URL and API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeInstanceService()
			id := uuid.New()
			svc.instances[id] = instance.Instance{
				ID:        id,
				Name:      "Test",
				AppType:   tt.appType,
				BaseURL:   tt.baseURL,
				APIKey:    "key",
				TimeoutMs: 30000,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			arrSvc := &fakeArrService{testErr: tt.testErr}
			handler := NewRouter(svc, arrSvc, nil, nil, nil).Handler()

			req := httptest.NewRequest(http.MethodPost, "/instances/"+id.String()+"/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			wantCode := tt.wantCode
			if wantCode == 0 {
				wantCode = http.StatusOK
			}
			if w.Code != wantCode {
				t.Errorf("status = %d, want %d", w.Code, wantCode)
			}

			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response: %v", err)
			}
			if body["status"] != tt.wantStatus {
				t.Errorf("status = %q, want %q", body["status"], tt.wantStatus)
			}
			if tt.wantMsg != "" && body["message"] != tt.wantMsg {
				t.Errorf("message = %q, want %q", body["message"], tt.wantMsg)
			}
		})
	}
}
