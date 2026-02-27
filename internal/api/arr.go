package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/instance"
)

// maxSearchBatchSize is the upper bound for a single search request.
const maxSearchBatchSize = 1000

type searchRequest struct {
	BatchSize int `json:"batchSize"`
}

// handleArrStatus returns the connection status of all *arr instances.
func (rt *Router) handleArrStatus(w http.ResponseWriter, r *http.Request) {
	statuses, err := rt.arr.Status(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch arr status")
		writeError(w, http.StatusInternalServerError, "failed to fetch arr status")
		return
	}
	writeJSON(w, http.StatusOK, statuses)
}

// handleInstanceSearch triggers a search cycle on the specified instance.
func (rt *Router) handleInstanceSearch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}

	// An empty body is accepted: batchSize defaults to 50 below when the
	// caller does not specify one.
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 50
	}
	if req.BatchSize > maxSearchBatchSize {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("batchSize must not exceed %d", maxSearchBatchSize))
		return
	}

	searched, err := rt.arr.SearchCycle(r.Context(), id, req.BatchSize)
	if err != nil {
		if errors.Is(err, instance.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		log.Error().Err(err).Msg("failed to run search cycle")
		writeError(w, http.StatusInternalServerError, "failed to run search cycle")
		return
	}

	writeJSON(w, http.StatusOK, searchResponse{Searched: searched})
}
