package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/settings"
)

type fakeSettingsService struct {
	resolved         settings.Resolved
	setErr           error
	setCalls         []setCall
	setBatchCalls    []setBatchCall
	removeErr        error
	removeCalls      []removeCall
	removeBatchCalls []removeBatchCall
}

type setCall struct {
	instanceID *uuid.UUID
	key        string
	value      string
}

type setBatchCall struct {
	instanceID *uuid.UUID
	entries    []settings.SettingEntry
}

type removeCall struct {
	instanceID *uuid.UUID
	key        string
}

type removeBatchCall struct {
	instanceID *uuid.UUID
	keys       []string
}

func (f *fakeSettingsService) ResolveGlobal(_ context.Context) (settings.Resolved, error) {
	return f.resolved, nil
}

func (f *fakeSettingsService) Resolve(_ context.Context, _ uuid.UUID) (settings.Resolved, error) {
	return f.resolved, nil
}

func (f *fakeSettingsService) Set(_ context.Context, instanceID *uuid.UUID, key, value string) error {
	f.setCalls = append(f.setCalls, setCall{instanceID, key, value})
	return f.setErr
}

func (f *fakeSettingsService) SetBatch(_ context.Context, instanceID *uuid.UUID, entries []settings.SettingEntry) error {
	f.setBatchCalls = append(f.setBatchCalls, setBatchCall{instanceID, entries})
	return f.setErr
}

func (f *fakeSettingsService) Remove(_ context.Context, instanceID *uuid.UUID, key string) error {
	f.removeCalls = append(f.removeCalls, removeCall{instanceID, key})
	return f.removeErr
}

func (f *fakeSettingsService) RemoveBatch(_ context.Context, instanceID *uuid.UUID, keys []string) error {
	f.removeBatchCalls = append(f.removeBatchCalls, removeBatchCall{instanceID, keys})
	return f.removeErr
}

func TestHandleGetSettingsGlobal(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{resolved: settings.Defaults()}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body struct {
		BatchSize      int    `json:"batchSize"`
		CooldownPeriod string `json:"cooldownPeriod"`
		SearchInterval string `json:"searchInterval"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.BatchSize != settings.Defaults().BatchSize {
		t.Errorf("BatchSize = %d, want %d", body.BatchSize, settings.Defaults().BatchSize)
	}
	if body.CooldownPeriod != "24h" {
		t.Errorf("CooldownPeriod = %q, want %q", body.CooldownPeriod, "24h")
	}
	if body.SearchInterval != "30m" {
		t.Errorf("SearchInterval = %q, want %q", body.SearchInterval, "30m")
	}
}

func TestHandleGetSettingsPerInstance(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{resolved: settings.Defaults()}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/settings?instanceId="+id.String(), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetSettingsBadUUID(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{resolved: settings.Defaults()}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/settings?instanceId=not-a-uuid", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSettings(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	body := `{"settings":[{"key":"batch_size","value":"20"}]}`
	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(svc.setBatchCalls) != 1 {
		t.Fatalf("setBatchCalls = %d, want 1", len(svc.setBatchCalls))
	}
	entries := svc.setBatchCalls[0].entries
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Key != "batch_size" || entries[0].Value != "20" {
		t.Errorf("entry = %+v, want batch_size=20", entries[0])
	}
}

func TestHandleUpdateSettingsInvalidKey(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{setErr: settings.ErrUnknownKey}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	body := `{"settings":[{"key":"bad_key","value":"x"}]}`
	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSettingsInvalidBody(t *testing.T) {
	t.Parallel()
	handler := NewRouter(nil, nil, &fakeSettingsService{}, nil, nil).Handler()

	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSettingsInternalError(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{setErr: errors.New("database down")}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	body := `{"settings":[{"key":"batch_size","value":"20"}]}`
	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleDeleteSettings(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	id := uuid.New()
	body := `{"keys":["batch_size","cooldown_period"]}`
	req := httptest.NewRequest(http.MethodDelete, "/settings?instanceId="+id.String(),
		bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(svc.removeBatchCalls) != 1 {
		t.Fatalf("removeBatchCalls = %d, want 1", len(svc.removeBatchCalls))
	}
	keys := svc.removeBatchCalls[0].keys
	if len(keys) != 2 {
		t.Fatalf("keys = %d, want 2", len(keys))
	}
	if keys[0] != "batch_size" {
		t.Errorf("keys[0] = %q, want batch_size", keys[0])
	}
	if keys[1] != "cooldown_period" {
		t.Errorf("keys[1] = %q, want cooldown_period", keys[1])
	}
}

func TestHandleDeleteSettingsEmptyKeys(t *testing.T) {
	t.Parallel()
	handler := NewRouter(nil, nil, &fakeSettingsService{}, nil, nil).Handler()

	body := `{"keys":[]}`
	req := httptest.NewRequest(http.MethodDelete, "/settings",
		bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleDeleteSettingsUnknownKey(t *testing.T) {
	t.Parallel()
	svc := &fakeSettingsService{removeErr: settings.ErrUnknownKey}
	handler := NewRouter(nil, nil, svc, nil, nil).Handler()

	body := `{"keys":["bad_key"]}`
	req := httptest.NewRequest(http.MethodDelete, "/settings",
		bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
