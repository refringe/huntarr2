package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/instance"
)

type createInstanceRequest struct {
	Name      string `json:"name"`
	AppType   string `json:"appType"`
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
	TimeoutMs int    `json:"timeoutMs"`
}

type updateInstanceRequest struct {
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
	TimeoutMs int    `json:"timeoutMs"`
}

type instanceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AppType   string `json:"appType"`
	BaseURL   string `json:"baseUrl"`
	TimeoutMs int    `json:"timeoutMs"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func toInstanceResponse(inst instance.Instance) instanceResponse {
	return instanceResponse{
		ID:        inst.ID.String(),
		Name:      inst.Name,
		AppType:   string(inst.AppType),
		BaseURL:   inst.BaseURL,
		TimeoutMs: inst.TimeoutMs,
		CreatedAt: inst.CreatedAt.Format(time.RFC3339),
		UpdatedAt: inst.UpdatedAt.Format(time.RFC3339),
	}
}

func (rt *Router) handleListInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := rt.instances.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to list instances")
		writeError(w, http.StatusInternalServerError, "failed to list instances")
		return
	}

	resp := make([]instanceResponse, len(instances))
	for i, inst := range instances {
		resp[i] = toInstanceResponse(inst)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (rt *Router) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req createInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst := &instance.Instance{
		Name:      req.Name,
		AppType:   instance.AppType(req.AppType),
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		TimeoutMs: req.TimeoutMs,
	}

	if err := rt.instances.Create(r.Context(), inst); err != nil {
		if ve, ok := errors.AsType[*instance.ValidationError](err); ok {
			writeError(w, http.StatusBadRequest, ve.Error())
			return
		}
		log.Error().Err(err).Msg("failed to create instance")
		writeError(w, http.StatusInternalServerError, "failed to create instance")
		return
	}

	writeJSON(w, http.StatusCreated, toInstanceResponse(*inst))
}

func (rt *Router) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}

	inst, err := rt.instances.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, instance.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		log.Error().Err(err).Msg("failed to get instance")
		writeError(w, http.StatusInternalServerError, "failed to get instance")
		return
	}

	writeJSON(w, http.StatusOK, toInstanceResponse(inst))
}

func (rt *Router) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}

	var req updateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst := &instance.Instance{
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		TimeoutMs: req.TimeoutMs,
	}

	updated, err := rt.instances.Update(r.Context(), id, inst)
	if err != nil {
		if ve, ok := errors.AsType[*instance.ValidationError](err); ok {
			writeError(w, http.StatusBadRequest, ve.Error())
			return
		}
		if errors.Is(err, instance.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		log.Error().Err(err).Msg("failed to update instance")
		writeError(w, http.StatusInternalServerError, "failed to update instance")
		return
	}

	writeJSON(w, http.StatusOK, toInstanceResponse(updated))
}

func (rt *Router) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}

	if err := rt.instances.Delete(r.Context(), id); err != nil {
		if errors.Is(err, instance.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		log.Error().Err(err).Msg("failed to delete instance")
		writeError(w, http.StatusInternalServerError, "failed to delete instance")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (rt *Router) handleTestInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}

	inst, err := rt.instances.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, instance.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		log.Error().Err(err).Msg("failed to get instance")
		writeError(w, http.StatusInternalServerError, "failed to get instance")
		return
	}

	if !inst.AppType.Valid() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown application type: %s", inst.AppType))
		return
	}

	testErr := rt.arr.TestConnection(r.Context(), inst.AppType, inst.BaseURL, inst.APIKey, inst.TimeoutMs)
	if testErr != nil {
		log.Warn().Err(testErr).Str("instance", id.String()).Msg("connection test failed")
		writeJSON(w, http.StatusBadGateway, statusResponse{
			Status:  "failed",
			Message: "connection test failed; check the instance URL and API key",
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}
