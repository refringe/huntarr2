package api

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/scheduler"
)

type fakeSchedulerStatus struct {
	status scheduler.Status
}

func (f *fakeSchedulerStatus) Status() scheduler.Status {
	return f.status
}

func TestHandleSchedulerStatus(t *testing.T) {
	t.Parallel()
	instID := uuid.New()
	fake := &fakeSchedulerStatus{
		status: scheduler.Status{
			Running:          true,
			SearchesThisHour: 42,
			Instances: []scheduler.InstanceSchedule{
				{
					InstanceID:   instID,
					InstanceName: "Sonarr",
					NextSearchAt: time.Now().Add(time.Hour),
					Enabled:      true,
				},
			},
		},
	}

	handler := NewRouter(nil, nil, nil, nil, fake).Handler()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scheduler/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body scheduler.Status
	if err := json.UnmarshalRead(w.Body, &body, jsonv1.FormatDurationAsNano(true)); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !body.Running {
		t.Error("Running = false, want true")
	}
	if body.SearchesThisHour != 42 {
		t.Errorf("SearchesThisHour = %d, want 42", body.SearchesThisHour)
	}
}
