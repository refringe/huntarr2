package api

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/settings"
)

type settingsUpdateRequest struct {
	Settings []settingsEntry `json:"settings"`
}

type settingsEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type settingsDeleteRequest struct {
	Keys []string `json:"keys"`
}

// settingsResponse is the JSON form of Resolved, with durations rendered as Go duration strings.
type settingsResponse struct {
	BatchSize         int    `json:"batchSize"`
	CooldownPeriod    string `json:"cooldownPeriod"`
	SearchWindowStart string `json:"searchWindowStart"`
	SearchWindowEnd   string `json:"searchWindowEnd"`
	SearchInterval    string `json:"searchInterval"`
	SearchLimit       int    `json:"searchLimit"`
	Enabled           bool   `json:"enabled"`
	SearchMissing     bool   `json:"searchMissing"`
}

func toSettingsResponse(r settings.Resolved) settingsResponse {
	return settingsResponse{
		BatchSize:         r.BatchSize,
		CooldownPeriod:    formatDuration(r.CooldownPeriod),
		SearchWindowStart: r.SearchWindowStart,
		SearchWindowEnd:   r.SearchWindowEnd,
		SearchInterval:    formatDuration(r.SearchInterval),
		SearchLimit:       r.SearchLimit,
		Enabled:           r.Enabled,
		SearchMissing:     r.SearchMissing,
	}
}

// formatDuration renders a time.Duration as a compact string (e.g. "24h", "1h30m", "45s", "500ms").
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	if s > 0 {
		fmt.Fprintf(&b, "%ds", s)
	}
	// A sub-second duration produces no output from the h/m/s branches and is rendered as milliseconds.
	if b.Len() == 0 && d > 0 {
		fmt.Fprintf(&b, "%dms", d.Milliseconds())
	}
	return b.String()
}

func (rt *Router) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rawID := r.URL.Query().Get("instanceId")
	if rawID != "" {
		id, ok := parseUUID(w, rawID)
		if !ok {
			return
		}
		resolved, err := rt.settings.Resolve(ctx, id)
		if err != nil {
			log.Error().Err(err).Msg("failed to resolve settings")
			writeError(w, http.StatusInternalServerError, "failed to resolve settings")
			return
		}
		writeJSON(w, http.StatusOK, toSettingsResponse(resolved))
		return
	}

	resolved, err := rt.settings.ResolveGlobal(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to resolve global settings")
		writeError(w, http.StatusInternalServerError, "failed to resolve settings")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsResponse(resolved))
}

func (rt *Router) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsUpdateRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	var instanceID *uuid.UUID

	rawID := r.URL.Query().Get("instanceId")
	if rawID != "" {
		id, ok := parseUUID(w, rawID)
		if !ok {
			return
		}
		instanceID = &id
	}

	entries := make([]settings.SettingEntry, len(req.Settings))
	for i, e := range req.Settings {
		entries[i] = settings.SettingEntry{Key: e.Key, Value: e.Value}
	}

	if err := rt.settings.SetBatch(ctx, instanceID, entries); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) || errors.Is(err, settings.ErrInvalidValue) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("failed to update settings")
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "updated"})
}

func (rt *Router) handleDeleteSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsDeleteRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Keys) == 0 {
		writeError(w, http.StatusBadRequest, "keys must not be empty")
		return
	}

	ctx := r.Context()
	var instanceID *uuid.UUID

	rawID := r.URL.Query().Get("instanceId")
	if rawID != "" {
		id, ok := parseUUID(w, rawID)
		if !ok {
			return
		}
		instanceID = &id
	}

	if err := rt.settings.RemoveBatch(ctx, instanceID, req.Keys); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("failed to remove settings")
		writeError(w, http.StatusInternalServerError,
			"failed to remove settings")
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "deleted"})
}
