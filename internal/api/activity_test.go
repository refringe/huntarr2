package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/activity"
)

type fakeActivityService struct {
	entries []activity.Entry
}

func (f *fakeActivityService) List(_ context.Context, params activity.ListParams) ([]activity.Entry, error) {
	out := make([]activity.Entry, 0, len(f.entries))
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
		out = append(out, e)
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > len(out) {
		limit = len(out)
	}
	return out[:limit], nil
}

func (f *fakeActivityService) Count(_ context.Context, params activity.ListParams) (int, error) {
	filtered, _ := f.List(context.Background(), activity.ListParams{
		Level:      params.Level,
		InstanceID: params.InstanceID,
		Action:     params.Action,
		Search:     params.Search,
		Since:      params.Since,
		Until:      params.Until,
	})
	return len(filtered), nil
}

func (f *fakeActivityService) Stats(_ context.Context, _ *time.Time) ([]activity.ActionStats, error) {
	return nil, nil
}

func TestHandleListActivity(t *testing.T) {
	t.Parallel()
	instID := uuid.New()
	svc := &fakeActivityService{
		entries: []activity.Entry{
			{
				ID:         uuid.New(),
				InstanceID: &instID,
				Level:      activity.LevelInfo,
				Action:     activity.ActionSearchCycle,
				Message:    "searched 5 items",
				CreatedAt:  time.Now(),
			},
		},
	}

	handler := NewRouter(nil, nil, nil, svc, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body activityListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body.Entries) != 1 {
		t.Errorf("entries = %d, want 1", len(body.Entries))
	}
	if body.Total != 1 {
		t.Errorf("total = %d, want 1", body.Total)
	}
}

func TestHandleListActivityFiltered(t *testing.T) {
	t.Parallel()
	svc := &fakeActivityService{
		entries: []activity.Entry{
			{ID: uuid.New(), Level: activity.LevelInfo, Action: activity.ActionSearchCycle},
			{ID: uuid.New(), Level: activity.LevelError, Action: activity.ActionSearchCycle},
			{ID: uuid.New(), Level: activity.LevelInfo, Action: activity.ActionSearchSkip},
		},
	}

	handler := NewRouter(nil, nil, nil, svc, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity?level=info&action=search_cycle", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body activityListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body.Entries) != 1 {
		t.Errorf("entries = %d, want 1", len(body.Entries))
	}
}

func TestHandleListActivityBadUUID(t *testing.T) {
	t.Parallel()
	svc := &fakeActivityService{}
	handler := NewRouter(nil, nil, nil, svc, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity?instanceId=not-a-uuid", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleListActivityBadTimestamp(t *testing.T) {
	t.Parallel()
	svc := &fakeActivityService{}
	handler := NewRouter(nil, nil, nil, svc, nil).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity?since=not-a-time", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
