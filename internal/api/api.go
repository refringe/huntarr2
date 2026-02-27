// Package api provides REST API handlers for the Huntarr2 web interface and
// external automation.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/internal/scheduler"
	"github.com/refringe/huntarr2/internal/settings"
)

type instanceService interface {
	List(ctx context.Context) ([]instance.Instance, error)
	Get(ctx context.Context, id uuid.UUID) (instance.Instance, error)
	Create(ctx context.Context, inst *instance.Instance) error
	Update(ctx context.Context, id uuid.UUID, inst *instance.Instance) (instance.Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type arrService interface {
	Status(ctx context.Context) ([]arr.InstanceStatus, error)
	TestConnection(ctx context.Context, appType instance.AppType, baseURL, apiKey string, timeoutMs int) error
	SearchCycle(ctx context.Context, instanceID uuid.UUID, batchSize int) (int, error)
}

type settingsService interface {
	ResolveGlobal(ctx context.Context) (settings.Resolved, error)
	Resolve(ctx context.Context, instanceID uuid.UUID) (settings.Resolved, error)
	Set(ctx context.Context, instanceID *uuid.UUID, key, value string) error
	SetBatch(ctx context.Context, instanceID *uuid.UUID, entries []settings.SettingEntry) error
	Remove(ctx context.Context, instanceID *uuid.UUID, key string) error
	RemoveBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error
}

type activityService interface {
	List(ctx context.Context, params activity.ListParams) ([]activity.Entry, error)
	Count(ctx context.Context, params activity.ListParams) (int, error)
	Stats(ctx context.Context, since *time.Time) ([]activity.ActionStats, error)
}

type schedulerService interface {
	Status() scheduler.Status
}

// Router holds API handler dependencies and a pre-built HTTP mux.
type Router struct {
	instances instanceService
	arr       arrService
	settings  settingsService
	activity  activityService
	scheduler schedulerService
	mux       *http.ServeMux
}

// NewRouter returns a Router with all API routes registered.
func NewRouter(
	instances instanceService,
	arr arrService,
	settings settingsService,
	activity activityService,
	scheduler schedulerService,
) *Router {
	rt := &Router{
		instances: instances,
		arr:       arr,
		settings:  settings,
		activity:  activity,
		scheduler: scheduler,
		mux:       http.NewServeMux(),
	}

	rt.mux.HandleFunc("GET /health", handleHealth)

	rt.mux.HandleFunc("GET /instances", rt.handleListInstances)
	rt.mux.HandleFunc("POST /instances", rt.handleCreateInstance)
	rt.mux.HandleFunc("GET /instances/{id}", rt.handleGetInstance)
	rt.mux.HandleFunc("PUT /instances/{id}", rt.handleUpdateInstance)
	rt.mux.HandleFunc("DELETE /instances/{id}", rt.handleDeleteInstance)
	rt.mux.HandleFunc("POST /instances/{id}/test", rt.handleTestInstance)

	rt.mux.HandleFunc("GET /arr/status", rt.handleArrStatus)
	rt.mux.HandleFunc("POST /instances/{id}/search", rt.handleInstanceSearch)

	rt.mux.HandleFunc("GET /settings", rt.handleGetSettings)
	rt.mux.HandleFunc("PUT /settings", rt.handleUpdateSettings)
	rt.mux.HandleFunc("DELETE /settings", rt.handleDeleteSettings)

	rt.mux.HandleFunc("GET /activity", rt.handleListActivity)

	rt.mux.HandleFunc("GET /scheduler/status", rt.handleSchedulerStatus)

	return rt
}

// Handler returns the pre-built HTTP handler for the API routes.
func (rt *Router) Handler() http.Handler {
	return rt.mux
}
