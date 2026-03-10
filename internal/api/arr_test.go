package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/instance"
)

type fakeArrService struct {
	statuses  []arr.InstanceStatus
	statusErr error
	testErr   error
	searched  int
	searchErr error
	lastBatch int
}

func (f *fakeArrService) Status(_ context.Context) ([]arr.InstanceStatus, error) {
	return f.statuses, f.statusErr
}

func (f *fakeArrService) TestConnection(_ context.Context, _ instance.AppType, _, _ string, _ int) error {
	return f.testErr
}

func (f *fakeArrService) SearchCycle(_ context.Context, _ uuid.UUID, batchSize int) (int, error) {
	f.lastBatch = batchSize
	return f.searched, f.searchErr
}

func TestHandleArrStatus(t *testing.T) {
	t.Parallel()
	arrSvc := &fakeArrService{
		statuses: []arr.InstanceStatus{
			{
				ID:        uuid.New(),
				Name:      "Sonarr 1",
				AppType:   "sonarr",
				Connected: true,
				Version:   "4.0.0",
			},
		},
	}
	handler := NewRouter(nil, arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/arr/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body []arr.InstanceStatus
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("len = %d, want 1", len(body))
	}
}

func TestHandleInstanceSearch(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	arrSvc := &fakeArrService{searched: 5}
	svc := newFakeInstanceService()
	svc.instances[id] = instance.Instance{ID: id, AppType: instance.AppTypeSonarr}
	handler := NewRouter(svc, arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/instances/"+id.String()+"/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]int
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["searched"] != 5 {
		t.Errorf("searched = %d, want 5", body["searched"])
	}
}

func TestHandleInstanceSearchNotFound(t *testing.T) {
	t.Parallel()
	arrSvc := &fakeArrService{searchErr: instance.ErrNotFound}
	handler := NewRouter(newFakeInstanceService(), arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/instances/"+uuid.New().String()+"/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleInstanceSearchDefaultBatchSize(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	arrSvc := &fakeArrService{searched: 3}
	svc := newFakeInstanceService()
	svc.instances[id] = instance.Instance{ID: id, AppType: instance.AppTypeSonarr}
	handler := NewRouter(svc, arrSvc, nil, nil, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/instances/"+id.String()+"/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if arrSvc.lastBatch != 50 {
		t.Errorf("batchSize = %d, want 50", arrSvc.lastBatch)
	}
}

func TestHandleInstanceSearchCustomBatchSize(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	arrSvc := &fakeArrService{searched: 10}
	svc := newFakeInstanceService()
	svc.instances[id] = instance.Instance{ID: id, AppType: instance.AppTypeSonarr}
	handler := NewRouter(svc, arrSvc, nil, nil, nil).Handler()

	body := `{"batchSize":25}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/instances/"+id.String()+"/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Length", "16")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if arrSvc.lastBatch != 25 {
		t.Errorf("batchSize = %d, want 25", arrSvc.lastBatch)
	}
}
