package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// statusResponse is returned by endpoints that confirm an operation (health, settings update, settings delete).
type statusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Status values used in statusResponse payloads.
const (
	statusOK     = "ok"
	statusFailed = "failed"
)

// searchResponse is returned by the search endpoint.
type searchResponse struct {
	Searched int `json:"searched"`
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// writeError writes a JSON error response with the given status and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseUUID extracts a UUID from raw, writing a 400 error and returning false on failure.
func parseUUID(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid UUID")
		return uuid.Nil, false
	}
	return id, true
}

// pathUUID extracts the "id" path parameter as a UUID, writing a 400 error on failure.
func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return parseUUID(w, r.PathValue("id"))
}
