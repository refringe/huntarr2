package arr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// historyResponse mirrors the paginated JSON envelope returned by all *arr
// history endpoints.
type historyResponse struct {
	Records []historyRecordResponse `json:"records"`
}

// historyEntityRef captures the titleSlug from an entity object embedded in
// a history record (e.g. movie, series, or artist). Only the slug is needed
// to construct the detail page URL.
type historyEntityRef struct {
	TitleSlug string `json:"titleSlug"`
}

// historyRecordResponse mirrors a single history record in the *arr JSON
// response. The per-app item ID fields (EpisodeID, MovieID, AlbumID) are
// only populated for the relevant application type; the others remain zero.
// The Movie, Series, and Artist fields are populated by the *arr API when
// the corresponding entity is associated with the history event.
type historyRecordResponse struct {
	ID        int               `json:"id"`
	Date      time.Time         `json:"date"`
	EventType string            `json:"eventType"`
	Data      map[string]string `json:"data"`

	// SourceTitle is the release name visible in all *arr history views.
	SourceTitle string `json:"sourceTitle"`

	// Quality wraps the nested quality object common to all *arr types.
	Quality struct {
		Quality struct {
			Name string `json:"name"`
		} `json:"quality"`
	} `json:"quality"`

	EpisodeID int `json:"episodeId"`
	MovieID   int `json:"movieId"`
	AlbumID   int `json:"albumId"`

	Movie  *historyEntityRef `json:"movie"`
	Series *historyEntityRef `json:"series"`
	Artist *historyEntityRef `json:"artist"`
}

// itemID returns the value of the named item ID field. Supported field
// names are "episodeId", "movieId", and "albumId".
func (r *historyRecordResponse) itemID(field string) int {
	switch field {
	case "episodeId":
		return r.EpisodeID
	case "movieId":
		return r.MovieID
	case "albumId":
		return r.AlbumID
	default:
		return 0
	}
}

// detailPath returns the URL path to the item's detail page in the *arr
// UI, derived from the embedded entity. Only one entity type is populated
// per record. Returns an empty string when no slug is available.
func (r *historyRecordResponse) detailPath() string {
	switch {
	case r.Movie != nil && r.Movie.TitleSlug != "":
		return "/movie/" + r.Movie.TitleSlug
	case r.Series != nil && r.Series.TitleSlug != "":
		return "/series/" + r.Series.TitleSlug
	case r.Artist != nil && r.Artist.TitleSlug != "":
		return "/artist/" + r.Artist.TitleSlug
	default:
		return ""
	}
}

// fetchEventPage queries the *arr history endpoint for a single event type
// and returns the raw records. The eventType parameter is the integer enum
// value defined by each *arr application (e.g. 3 for downloadFolderImported
// in Sonarr).
func fetchEventPage(
	ctx context.Context,
	c *client,
	apiVersion string,
	eventType int,
	pageSize int,
) ([]historyRecordResponse, error) {
	params := url.Values{}
	params.Set("eventType", strconv.Itoa(eventType))
	params.Set("page", "1")
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("sortDirection", "descending")
	params.Set("sortKey", "date")
	path := fmt.Sprintf("/api/%s/history", apiVersion) + "?" + params.Encode()

	var raw historyResponse
	if err := c.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	return raw.Records, nil
}

// fetchArrHistory queries the history endpoint for import events and,
// separately, for file-deleted events that carry a "reason":"upgrade" flag.
// The upgrade flag is set by cross-referencing item IDs from the delete
// query against the import records.
//
// The delete event fetch is non-fatal: if it fails, all imports are still
// tracked as new downloads. Only the upgrade/download distinction is lost.
//
// deleteEventType and importEventType are the integer enum values for the
// relevant history event types in the *arr API (e.g. Sonarr uses 5 for
// episodeFileDeleted and 3 for downloadFolderImported). itemIDField
// selects which per-app ID field to compare (e.g. "episodeId").
func fetchArrHistory(
	ctx context.Context,
	c *client,
	apiVersion string,
	since time.Time,
	pageSize int,
	deleteEventType int,
	importEventType int,
	itemIDField string,
) ([]HistoryRecord, error) {
	upgradedItems := make(map[int]bool)
	deleteRecords, err := fetchEventPage(ctx, c, apiVersion, deleteEventType, pageSize)
	if err != nil {
		log.Warn().Err(err).
			Int("eventType", deleteEventType).
			Msg("unable to fetch delete events; upgrade detection disabled for this poll")
	} else {
		for _, r := range deleteRecords {
			if strings.EqualFold(r.Data["reason"], "upgrade") && r.Date.After(since) {
				upgradedItems[r.itemID(itemIDField)] = true
			}
		}
		log.Debug().
			Int("deleteRecords", len(deleteRecords)).
			Int("upgradesDetected", len(upgradedItems)).
			Msg("history: processed delete events")
	}

	importRecords, err := fetchEventPage(ctx, c, apiVersion, importEventType, pageSize)
	if err != nil {
		return nil, fmt.Errorf("fetching import events: %w", err)
	}

	var records []HistoryRecord
	for _, r := range importRecords {
		if !r.Date.After(since) {
			continue
		}
		records = append(records, HistoryRecord{
			ID:         r.ID,
			Date:       r.Date,
			ItemLabel:  r.SourceTitle,
			DetailPath: r.detailPath(),
			IsUpgrade:  upgradedItems[r.itemID(itemIDField)],
			Quality:    r.Quality.Quality.Name,
		})
	}

	log.Debug().
		Int("importRecords", len(importRecords)).
		Int("afterFilter", len(records)).
		Time("since", since).
		Msg("history: processed import events")

	return records, nil
}
