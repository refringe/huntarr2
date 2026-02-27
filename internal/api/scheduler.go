package api

import "net/http"

// handleSchedulerStatus returns the current scheduler state.
func (rt *Router) handleSchedulerStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, rt.scheduler.Status())
}
