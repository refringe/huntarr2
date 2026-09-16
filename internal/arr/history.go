package arr

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// historyResponse mirrors the paginated JSON envelope returned by all *arr history endpoints.
type historyResponse struct {
	Records []historyRecordResponse `json:"records"`
}

// historyEntityRef captures the titleSlug from an entity object (movie, series, or artist) embedded in a history
// record.
type historyEntityRef struct {
	TitleSlug string `json:"titleSlug"`
}

// historyRecordResponse mirrors a single history record in the *arr JSON response. The per-app item ID fields
// (EpisodeID, MovieID, AlbumID) and the Movie, Series, and Artist entities are populated only for the relevant
// application type; the others remain zero.
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

// itemID returns the value of the named item ID field ("episodeId", "movieId", or "albumId").
func (r *historyRecordResponse) itemID(field string) int {
	switch field {
	case historyFieldEpisode:
		return r.EpisodeID
	case historyFieldMovie:
		return r.MovieID
	case historyFieldAlbum:
		return r.AlbumID
	default:
		return 0
	}
}

// detailPath returns the *arr UI detail page path from the populated entity, or empty when no slug is available.
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

// fetchEventPage queries the *arr history endpoint for a single event type, given the integer enum value defined
// by each *arr application (e.g. 3 for downloadFolderImported in Sonarr).
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

// fetchArrHistory queries the history endpoint for import events and, separately, for file-deleted events carrying
// a "reason":"upgrade" flag, marking an import as an upgrade when its item ID also appears in the delete records.
// A failed delete fetch is non-fatal: every import is then tracked as a new download. deleteEventType and
// importEventTypes are the integer enum values used by the *arr API; one page is fetched per import type,
// duplicates are dropped by record ID, and the merged records are sorted newest first. itemIDField selects the
// per-app ID field to compare (e.g. "episodeId").
func fetchArrHistory(
	ctx context.Context,
	c *client,
	apiVersion string,
	since time.Time,
	pageSize int,
	deleteEventType int,
	importEventTypes []int,
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

	var records []HistoryRecord
	seen := make(map[int]bool)
	fetched := 0
	for _, eventType := range importEventTypes {
		importRecords, err := fetchEventPage(ctx, c, apiVersion, eventType, pageSize)
		if err != nil {
			return nil, fmt.Errorf("fetching import events (type %d): %w", eventType, err)
		}
		fetched += len(importRecords)

		for _, r := range importRecords {
			if !r.Date.After(since) || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			records = append(records, HistoryRecord{
				ID:         r.ID,
				Date:       r.Date,
				ItemLabel:  r.SourceTitle,
				DetailPath: r.detailPath(),
				IsUpgrade:  upgradedItems[r.itemID(itemIDField)],
				Quality:    r.Quality.Quality.Name,
			})
		}
	}

	// Each page arrives newest first, but records from separate pages interleave; restore newest-first ordering.
	slices.SortFunc(records, func(a, b HistoryRecord) int {
		if c := b.Date.Compare(a.Date); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})

	log.Debug().
		Int("importEventTypes", len(importEventTypes)).
		Int("importRecords", fetched).
		Int("afterFilter", len(records)).
		Time("since", since).
		Msg("history: processed import events")

	return records, nil
}
