package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// statusResponse is returned by endpoints that confirm an operation
// (health, settings update, settings delete, etc.).
type statusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// searchResponse is returned by the search endpoint.
type searchResponse struct {
	Searched int `json:"searched"`
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode error is logged but unrecoverable: the status header has already
	// been written and the client may have received partial data.
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// writeError writes a JSON error response with the given status and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseUUID extracts a UUID from raw and writes a 400 error on failure. The
// boolean return indicates whether parsing succeeded.
func parseUUID(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid UUID")
		return uuid.Nil, false
	}
	return id, true
}

// pathUUID extracts the "id" path parameter as a UUID and writes a 400 error
// on failure. It combines PathValue lookup with UUID parsing in a single call
// to reduce boilerplate in handlers that operate on a resource by ID.
func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return parseUUID(w, r.PathValue("id"))
}
