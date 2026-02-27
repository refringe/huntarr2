package arr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// historyResponse mirrors the paginated JSON envelope returned by all *arr
// history endpoints.
type historyResponse struct {
	Records []historyRecordResponse `json:"records"`
}

// historyRecordResponse mirrors a single history record in the *arr JSON
// response.
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
}

// fetchArrHistory queries the history endpoint for downloadFolderImported
// events and converts them into HistoryRecord values. The function works
// for all four *arr types because they share the same paginated response
// shape.
func fetchArrHistory(
	ctx context.Context,
	c *client,
	apiVersion string,
	since time.Time,
	pageSize int,
) ([]HistoryRecord, error) {
	params := url.Values{}
	params.Set("eventType", "downloadFolderImported")
	params.Set("page", "1")
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("sortDirection", "descending")
	params.Set("sortKey", "date")
	path := fmt.Sprintf("/api/%s/history", apiVersion) + "?" + params.Encode()

	var raw historyResponse
	if err := c.get(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("fetching history: %w", err)
	}

	var records []HistoryRecord
	for _, r := range raw.Records {
		if !r.Date.After(since) {
			continue
		}
		records = append(records, HistoryRecord{
			ID:        r.ID,
			Date:      r.Date,
			ItemLabel: r.SourceTitle,
			IsUpgrade: r.Data["reason"] == "upgrade",
			Quality:   r.Quality.Quality.Name,
		})
	}

	return records, nil
}
