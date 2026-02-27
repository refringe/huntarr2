package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/activity"
)

// maxActivityLimit is the maximum number of activity entries returned per
// request. The service layer also caps the limit, but enforcing here gives
// clients a clear 400 error rather than a silent clamp.
const maxActivityLimit = 500

// maxActivityOffset prevents unreasonably deep pagination that would
// strain the database.
const maxActivityOffset = 100000

type activityListResponse struct {
	Entries []activity.Entry `json:"entries"`
	Total   int              `json:"total"`
}

func (rt *Router) handleListActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := activity.ListParams{}

	if v := q.Get("level"); v != "" {
		lvl := activity.Level(v)
		if !activity.ValidLevel(lvl) {
			writeError(w, http.StatusBadRequest, "unrecognised level")
			return
		}
		params.Level = lvl
	}
	if v := q.Get("instanceId"); v != "" {
		id, ok := parseUUID(w, v)
		if !ok {
			return
		}
		params.InstanceID = &id
	}
	if v := q.Get("action"); v != "" {
		action := activity.Action(v)
		if !activity.ValidAction(action) {
			writeError(w, http.StatusBadRequest, "unrecognised action")
			return
		}
		params.Action = action
	}
	if v := q.Get("search"); v != "" {
		params.Search = v
	}
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since timestamp")
			return
		}
		params.Since = &t
	}
	if v := q.Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid until timestamp")
			return
		}
		params.Until = &t
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		if n > maxActivityLimit {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("limit must not exceed %d", maxActivityLimit))
			return
		}
		params.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		if n > maxActivityOffset {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("offset must not exceed %d", maxActivityOffset))
			return
		}
		params.Offset = n
	}

	ctx := r.Context()

	entries, err := rt.activity.List(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to list activity entries")
		writeError(w, http.StatusInternalServerError,
			"failed to list activity entries")
		return
	}

	total, err := rt.activity.Count(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("failed to count activity entries")
		writeError(w, http.StatusInternalServerError,
			"failed to count activity entries")
		return
	}

	writeJSON(w, http.StatusOK, activityListResponse{
		Entries: entries,
		Total:   total,
	})
}
