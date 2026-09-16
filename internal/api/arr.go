package api

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/instance"
)

// maxSearchBatchSize is the upper bound for a single search request.
const maxSearchBatchSize = 1000

// searchCycleWriteBudget extends the HTTP write deadline for a manual search cycle: it must cover the quality
// profile and search command requests plus the library fetch budget, with margin to write the response.
const searchCycleWriteBudget = 30 * time.Minute

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

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(searchCycleWriteBudget)); err != nil {
		log.Warn().Err(err).Msg("could not extend write deadline for search cycle")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// An empty body is accepted.
	var req searchRequest
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
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
