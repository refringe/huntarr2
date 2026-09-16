package arr

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// fetchRadarrLibrary retrieves all movies from a Radarr (or Whisparr v3) instance as LibraryItems. Whisparr v3
// models scenes as movies: its titleSlug is an identifier such as "tmdb:<id>" (still valid in the /movie/ detail
// path) and its year may be zero.
func fetchRadarrLibrary(
	ctx context.Context,
	c *client,
	apiVersion string,
) ([]LibraryItem, error) {
	var movies []struct {
		ID               int    `json:"id"`
		Title            string `json:"title"`
		Year             int    `json:"year"`
		TitleSlug        string `json:"titleSlug"`
		QualityProfileID int    `json:"qualityProfileId"`
		HasFile          bool   `json:"hasFile"`
		Monitored        bool   `json:"monitored"`
		MovieFile        *struct {
			Quality struct {
				Quality struct {
					ID int `json:"id"`
				} `json:"quality"`
			} `json:"quality"`
		} `json:"movieFile"`
	}

	path := fmt.Sprintf("/api/%s/movie", apiVersion)
	if err := c.get(ctx, path, &movies); err != nil {
		return nil, fmt.Errorf("fetching radarr library: %w", err)
	}

	items := make([]LibraryItem, len(movies))
	for i, m := range movies {
		var qualityIDs []int
		if m.MovieFile != nil {
			qualityIDs = []int{m.MovieFile.Quality.Quality.ID}
		}
		label := m.Title
		if m.Year != 0 {
			label = fmt.Sprintf("%s (%d)", m.Title, m.Year)
		}
		items[i] = LibraryItem{
			ID:                m.ID,
			Label:             label,
			DetailPath:        fmt.Sprintf("/movie/%s", m.TitleSlug),
			QualityProfileID:  m.QualityProfileID,
			CurrentQualityIDs: qualityIDs,
			HasFile:           m.HasFile,
			Monitored:         m.Monitored,
		}
	}
	return items, nil
}

// fetchSonarrLibrary retrieves all episodes from a Sonarr (or Whisparr v2) instance as LibraryItems, fetching all
// series first and then episodes per monitored series. Per-series failures are logged and skipped.
func fetchSonarrLibrary(
	ctx context.Context,
	c *client,
	apiVersion string,
) ([]LibraryItem, error) {
	var seriesList []struct {
		ID               int    `json:"id"`
		Title            string `json:"title"`
		TitleSlug        string `json:"titleSlug"`
		QualityProfileID int    `json:"qualityProfileId"`
		Monitored        bool   `json:"monitored"`
	}

	seriesAPIPath := fmt.Sprintf("/api/%s/series", apiVersion)
	if err := c.get(ctx, seriesAPIPath, &seriesList); err != nil {
		return nil, fmt.Errorf("fetching sonarr series: %w", err)
	}

	items := make([]LibraryItem, 0)
	skipped := 0
	for _, series := range seriesList {
		if !series.Monitored {
			continue
		}

		var episodes []struct {
			ID            int  `json:"id"`
			EpisodeNumber int  `json:"episodeNumber"`
			SeasonNumber  int  `json:"seasonNumber"`
			HasFile       bool `json:"hasFile"`
			Monitored     bool `json:"monitored"`
			EpisodeFile   *struct {
				Quality struct {
					Quality struct {
						ID int `json:"id"`
					} `json:"quality"`
				} `json:"quality"`
			} `json:"episodeFile"`
		}

		episodePath := fmt.Sprintf("/api/%s/episode?seriesId=%d&includeEpisodeFile=true",
			apiVersion, series.ID)
		if err := c.get(ctx, episodePath, &episodes); err != nil {
			// A cancelled or expired context would fail every remaining series and truncate the library.
			if ctx.Err() != nil {
				return nil, fmt.Errorf("fetching episodes for series %q: %w", series.Title, err)
			}
			skipped++
			log.Warn().Err(err).
				Int("seriesId", series.ID).
				Str("series", series.Title).
				Msg("failed to fetch episodes, skipping series")
			continue
		}

		detailPath := fmt.Sprintf("/series/%s", series.TitleSlug)
		for _, ep := range episodes {
			var qualityIDs []int
			if ep.EpisodeFile != nil {
				qualityIDs = []int{ep.EpisodeFile.Quality.Quality.ID}
			}
			items = append(items, LibraryItem{
				ID:                ep.ID,
				Label:             fmt.Sprintf("%s S%02dE%02d", series.Title, ep.SeasonNumber, ep.EpisodeNumber),
				DetailPath:        detailPath,
				QualityProfileID:  series.QualityProfileID,
				CurrentQualityIDs: qualityIDs,
				HasFile:           ep.HasFile,
				Monitored:         ep.Monitored,
			})
		}
	}

	if skipped > 0 {
		log.Warn().Int("skipped", skipped).
			Msg("sonarr library fetch skipped series after errors")
	}

	return items, nil
}

// fetchLidarrLibrary retrieves all albums from a Lidarr instance as LibraryItems, fetching track files for albums
// with files and storing every track quality ID. Per-album failures are logged and skipped.
func fetchLidarrLibrary(
	ctx context.Context,
	c *client,
	apiVersion string,
) ([]LibraryItem, error) {
	var albums []struct {
		ID        int    `json:"id"`
		Title     string `json:"title"`
		Monitored bool   `json:"monitored"`
		Artist    struct {
			ArtistName       string `json:"artistName"`
			TitleSlug        string `json:"titleSlug"`
			QualityProfileID int    `json:"qualityProfileId"`
		} `json:"artist"`
		Statistics struct {
			TrackFileCount int `json:"trackFileCount"`
		} `json:"statistics"`
	}

	albumPath := fmt.Sprintf("/api/%s/album", apiVersion)
	if err := c.get(ctx, albumPath, &albums); err != nil {
		return nil, fmt.Errorf("fetching lidarr albums: %w", err)
	}

	items := make([]LibraryItem, 0)
	skipped := 0
	for _, album := range albums {
		hasFile := album.Statistics.TrackFileCount > 0

		item := LibraryItem{
			ID:               album.ID,
			Label:            fmt.Sprintf("%s - %s", album.Artist.ArtistName, album.Title),
			DetailPath:       fmt.Sprintf("/artist/%s", album.Artist.TitleSlug),
			QualityProfileID: album.Artist.QualityProfileID,
			HasFile:          hasFile,
			Monitored:        album.Monitored,
		}

		if hasFile {
			var tracks []struct {
				Quality struct {
					Quality struct {
						ID int `json:"id"`
					} `json:"quality"`
				} `json:"quality"`
			}

			trackPath := fmt.Sprintf("/api/%s/trackfile?albumId=%d",
				apiVersion, album.ID)
			if err := c.get(ctx, trackPath, &tracks); err != nil {
				// A cancelled or expired context would fail every remaining album and truncate the library.
				if ctx.Err() != nil {
					return nil, fmt.Errorf("fetching track files for album %q: %w", album.Title, err)
				}
				skipped++
				log.Warn().Err(err).
					Int("albumId", album.ID).
					Str("album", album.Title).
					Msg("failed to fetch track files, skipping album")
				continue
			}

			qualityIDs := make([]int, len(tracks))
			for j, t := range tracks {
				qualityIDs[j] = t.Quality.Quality.ID
			}
			item.CurrentQualityIDs = qualityIDs
		}

		items = append(items, item)
	}

	if skipped > 0 {
		log.Warn().Int("skipped", skipped).
			Msg("lidarr library fetch skipped albums after errors")
	}

	return items, nil
}
