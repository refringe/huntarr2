package api

import "net/http"

// handleHealth returns a simple health status response used by monitoring
// tools and container orchestrators.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}
