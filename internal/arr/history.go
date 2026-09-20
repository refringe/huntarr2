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

const (
	coverTypePoster = "poster"
	coverTypeCover  = "cover"
)

const mediaCoverHeight = 250

var historyIncludeParams = []string{"includeMovie", "includeSeries", "includeEpisode", "includeArtist", "includeAlbum"}

// historyConfig holds an application's history event type IDs, item ID field name, and file endpoint name.
type historyConfig struct {
	deleteEventType  int
	importEventTypes []int
	itemIDField      string
	fileEndpoint     string
}

// historyResponse mirrors the paginated JSON envelope returned by all *arr history endpoints.
type historyResponse struct {
	Records []historyRecordResponse `json:"records"`
}

type historyImageRef struct {
	CoverType string `json:"coverType"`
}

// historyEntityRef mirrors the fields read from a movie, series, artist, or album embedded in a history record.
type historyEntityRef struct {
	ID         int               `json:"id"`
	TitleSlug  string            `json:"titleSlug"`
	Title      string            `json:"title"`
	Year       int               `json:"year"`
	ArtistName string            `json:"artistName"`
	Images     []historyImageRef `json:"images"`
}

// hasImage reports whether the entity's images list contains the given cover type.
func (e *historyEntityRef) hasImage(coverType string) bool {
	if e == nil {
		return false
	}
	for _, img := range e.Images {
		if strings.EqualFold(img.CoverType, coverType) {
			return true
		}
	}
	return false
}

// historyEpisodeRef captures the fields read from the episode object embedded in a Sonarr history record.
type historyEpisodeRef struct {
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

// historyRecordResponse mirrors a single history record in the *arr JSON response.
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

	CustomFormatScore *int `json:"customFormatScore"`

	EpisodeID int `json:"episodeId"`
	MovieID   int `json:"movieId"`
	AlbumID   int `json:"albumId"`

	Movie   *historyEntityRef  `json:"movie"`
	Series  *historyEntityRef  `json:"series"`
	Episode *historyEpisodeRef `json:"episode"`
	Artist  *historyEntityRef  `json:"artist"`
	Album   *historyEntityRef  `json:"album"`
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

// dataValue returns the named entry of the record's data map, matching the key case-insensitively.
func (r *historyRecordResponse) dataValue(key string) string {
	for k, v := range r.Data {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// dataInt64 returns the named data entry parsed as a 64-bit integer, or 0 when absent or malformed.
func (r *historyRecordResponse) dataInt64(key string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(r.dataValue(key)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// dataInt returns the named data entry parsed as an int, or 0 when absent, malformed, or out of range.
func (r *historyRecordResponse) dataInt(key string) int {
	n, err := strconv.Atoi(strings.TrimSpace(r.dataValue(key)))
	if err != nil {
		return 0
	}
	return n
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

// itemLabel returns a human-readable title built from the embedded entities, falling back to the release title.
func (r *historyRecordResponse) itemLabel() string {
	switch {
	case r.Movie != nil && r.Movie.Title != "":
		if r.Movie.Year != 0 {
			return fmt.Sprintf("%s (%d)", r.Movie.Title, r.Movie.Year)
		}
		return r.Movie.Title
	case r.Series != nil && r.Series.Title != "":
		if r.Episode != nil && (r.Episode.SeasonNumber != 0 || r.Episode.EpisodeNumber != 0) {
			return fmt.Sprintf("%s S%02dE%02d", r.Series.Title, r.Episode.SeasonNumber, r.Episode.EpisodeNumber)
		}
		return r.Series.Title
	case r.Album != nil && r.Album.Title != "":
		if r.Artist != nil && r.Artist.ArtistName != "" {
			return r.Artist.ArtistName + " - " + r.Album.Title
		}
		return r.Album.Title
	case r.Artist != nil && r.Artist.ArtistName != "":
		return r.Artist.ArtistName
	default:
		return r.SourceTitle
	}
}

// mediaCoverPath returns the poster path below the mediacover API for the populated entity, or empty without one.
func (r *historyRecordResponse) mediaCoverPath() string {
	switch {
	case r.Movie.hasImage(coverTypePoster):
		return fmt.Sprintf("%d/%s-%d.jpg", r.Movie.ID, coverTypePoster, mediaCoverHeight)
	case r.Series.hasImage(coverTypePoster):
		return fmt.Sprintf("%d/%s-%d.jpg", r.Series.ID, coverTypePoster, mediaCoverHeight)
	case r.Album.hasImage(coverTypeCover):
		return fmt.Sprintf("album/%d/%s-%d.jpg", r.Album.ID, coverTypeCover, mediaCoverHeight)
	case r.Artist.hasImage(coverTypePoster):
		return fmt.Sprintf("artist/%d/%s-%d.jpg", r.Artist.ID, coverTypePoster, mediaCoverHeight)
	default:
		return ""
	}
}

// deletedFile describes the file removed by an upgrade, read from a file-deleted history record.
type deletedFile struct {
	quality string
	size    int64
	score   *int
}

// fetchEventPage queries the history endpoint for one event type, given the application's integer enum value.
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
	for _, include := range historyIncludeParams {
		params.Set(include, "true")
	}
	path := fmt.Sprintf("/api/%s/history", apiVersion) + "?" + params.Encode()

	var raw historyResponse
	if err := c.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	return raw.Records, nil
}

// fetchArrHistory returns import records dated after since, flagged as upgrades by matching file-deleted records.
func fetchArrHistory(
	ctx context.Context,
	c *client,
	apiVersion string,
	since time.Time,
	pageSize int,
	cfg historyConfig,
) ([]HistoryRecord, error) {
	upgradedItems := make(map[int]deletedFile)
	deleteRecords, err := fetchEventPage(ctx, c, apiVersion, cfg.deleteEventType, pageSize)
	if err != nil {
		log.Warn().Err(err).
			Int("eventType", cfg.deleteEventType).
			Msg("unable to fetch delete events; upgrade detection disabled for this poll")
	} else {
		for _, r := range deleteRecords {
			if !strings.EqualFold(r.dataValue("reason"), "upgrade") || !r.Date.After(since) {
				continue
			}
			itemID := r.itemID(cfg.itemIDField)
			if _, seen := upgradedItems[itemID]; seen {
				continue
			}
			upgradedItems[itemID] = deletedFile{
				quality: r.Quality.Quality.Name,
				size:    r.dataInt64("size"),
				score:   r.CustomFormatScore,
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
	for _, eventType := range cfg.importEventTypes {
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
			rec := HistoryRecord{
				ID:                r.ID,
				ItemID:            r.itemID(cfg.itemIDField),
				Date:              r.Date,
				ItemLabel:         r.itemLabel(),
				ReleaseTitle:      r.SourceTitle,
				DetailPath:        r.detailPath(),
				MediaCoverPath:    r.mediaCoverPath(),
				Quality:           r.Quality.Quality.Name,
				Size:              r.dataInt64("size"),
				CustomFormatScore: r.CustomFormatScore,
				ReleaseGroup:      r.dataValue("releaseGroup"),
				FileID:            r.dataInt("fileId"),
			}
			if deleted, ok := upgradedItems[rec.ItemID]; ok {
				rec.IsUpgrade = true
				rec.PreviousQuality = deleted.quality
				rec.PreviousSize = deleted.size
				rec.PreviousCustomFormatScore = deleted.score
			}
			records = append(records, rec)
		}
	}

	// Each page arrives newest first, but records from separate pages interleave; restore newest-first ordering.
	slices.SortFunc(records, func(a, b HistoryRecord) int {
		if c := b.Date.Compare(a.Date); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	records = newestPerItem(records)

	for i := range records {
		if !records[i].IsUpgrade || records[i].FileID == 0 || ctx.Err() != nil {
			continue
		}
		file, err := fetchFile(ctx, c, apiVersion, cfg.fileEndpoint, records[i].FileID)
		if err != nil {
			log.Debug().Err(err).Int("fileId", records[i].FileID).
				Msg("history: unable to fetch imported file details")
			continue
		}
		records[i].MediaInfo = file.mediaInfo()
		if records[i].Size == 0 {
			records[i].Size = file.Size
		}
		if records[i].CustomFormatScore == nil {
			records[i].CustomFormatScore = file.CustomFormatScore
		}
		if records[i].ReleaseGroup == "" {
			records[i].ReleaseGroup = file.ReleaseGroup
		}
	}

	log.Debug().
		Int("importEventTypes", len(cfg.importEventTypes)).
		Int("importRecords", fetched).
		Int("afterFilter", len(records)).
		Time("since", since).
		Msg("history: processed import events")

	return records, nil
}

// newestPerItem keeps the first record per non-zero item ID from a newest-first list.
func newestPerItem(records []HistoryRecord) []HistoryRecord {
	seenItems := make(map[int]bool, len(records))
	return slices.DeleteFunc(records, func(r HistoryRecord) bool {
		if r.ItemID == 0 {
			return false
		}
		if seenItems[r.ItemID] {
			return true
		}
		seenItems[r.ItemID] = true
		return false
	})
}

// fileResponse mirrors the fields read from an *arr file resource (moviefile, episodefile, or trackfile).
type fileResponse struct {
	Size              int64              `json:"size"`
	ReleaseGroup      string             `json:"releaseGroup"`
	CustomFormatScore *int               `json:"customFormatScore"`
	MediaInfo         *mediaInfoResponse `json:"mediaInfo"`
}

// mediaInfoResponse mirrors the mediaInfo object of an *arr file resource.
type mediaInfoResponse struct {
	Resolution            string  `json:"resolution"`
	VideoCodec            string  `json:"videoCodec"`
	VideoBitDepth         int     `json:"videoBitDepth"`
	VideoBitrate          int64   `json:"videoBitrate"`
	VideoDynamicRangeType string  `json:"videoDynamicRangeType"`
	AudioCodec            string  `json:"audioCodec"`
	AudioChannels         float64 `json:"audioChannels"`
	AudioBitrate          int64   `json:"audioBitrate"`
	AudioBitrateText      string  `json:"audioBitRate"`
	AudioBits             string  `json:"audioBits"`
	AudioSampleRate       string  `json:"audioSampleRate"`
	RunTime               string  `json:"runTime"`
}

// mediaInfo converts the file's media info into the domain type, or nil when the file carries none.
func (f *fileResponse) mediaInfo() *MediaInfo {
	if f.MediaInfo == nil {
		return nil
	}
	m := MediaInfo(*f.MediaInfo)
	if m == (MediaInfo{}) {
		return nil
	}
	return &m
}

// fetchFile loads a single file resource (e.g. "/api/v3/moviefile/12") from an *arr instance.
func fetchFile(ctx context.Context, c *client, apiVersion, endpoint string, fileID int) (fileResponse, error) {
	var raw fileResponse
	path := fmt.Sprintf("/api/%s/%s/%d", apiVersion, endpoint, fileID)
	if err := c.get(ctx, path, &raw); err != nil {
		return fileResponse{}, err
	}
	return raw, nil
}
