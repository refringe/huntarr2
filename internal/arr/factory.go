package arr

import (
	"context"
	"fmt"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

// *arr history event type integer IDs. Each application defines its own
// enum; the values below are extracted from the respective source
// repositories and must match the server-side definitions.
const (
	// Sonarr / Whisparr (EpisodeHistoryEventType).
	sonarrImported    = 3 // downloadFolderImported
	sonarrFileDeleted = 5 // episodeFileDeleted

	// Radarr (MovieHistoryEventType).
	radarrImported    = 2 // downloadFolderImported
	radarrFileDeleted = 4 // movieFileDeleted

	// Lidarr (EntityHistoryEventType).
	lidarrImported    = 3 // downloadImported
	lidarrFileDeleted = 4 // trackFileDeleted
)

// Per-application command names, search ID field names, and history item
// ID field names referenced by appConfigs.
const (
	cmdEpisodeSearch = "EpisodeSearch"
	cmdMoviesSearch  = "MoviesSearch"
	cmdAlbumSearch   = "AlbumSearch"

	idFieldEpisodes = "episodeIds"
	idFieldMovies   = "movieIds"
	idFieldAlbums   = "albumIds"

	historyFieldEpisode = "episodeId"
	historyFieldMovie   = "movieId"
	historyFieldAlbum   = "albumId"
)

// newHistoryFunc returns a fetchHistoryFunc that binds the per-app event
// type IDs and item ID field into a closure matching the fetchHistoryFunc
// signature.
func newHistoryFunc(deleteEventType, importEventType int, itemIDField string) fetchHistoryFunc {
	return func(ctx context.Context, c *client, apiVersion string,
		since time.Time, pageSize int,
	) ([]HistoryRecord, error) {
		return fetchArrHistory(ctx, c, apiVersion, since, pageSize,
			deleteEventType, importEventType, itemIDField)
	}
}

// appConfigs maps each supported application type to its adapter
// configuration. The map is read-only after initialisation.
var appConfigs = map[instance.AppType]appConfig{
	instance.AppTypeSonarr: {
		name:         string(instance.AppTypeSonarr),
		apiVersion:   "v3",
		commandKey:   cmdEpisodeSearch,
		idField:      idFieldEpisodes,
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, sonarrImported, historyFieldEpisode),
	},
	instance.AppTypeRadarr: {
		name:         string(instance.AppTypeRadarr),
		apiVersion:   "v3",
		commandKey:   cmdMoviesSearch,
		idField:      idFieldMovies,
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(radarrFileDeleted, radarrImported, historyFieldMovie),
	},
	instance.AppTypeLidarr: {
		name:         string(instance.AppTypeLidarr),
		apiVersion:   "v1",
		commandKey:   cmdAlbumSearch,
		idField:      idFieldAlbums,
		fetchLibrary: fetchLidarrLibrary,
		fetchHistory: newHistoryFunc(lidarrFileDeleted, lidarrImported, historyFieldAlbum),
	},
	// Whisparr shares Sonarr's episode-based structure and API.
	instance.AppTypeWhisparr: {
		name:         string(instance.AppTypeWhisparr),
		apiVersion:   "v3",
		commandKey:   cmdEpisodeSearch,
		idField:      idFieldEpisodes,
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, sonarrImported, historyFieldEpisode),
	},
}

// NewApp constructs the appropriate App implementation for the given
// application type.
func NewApp(appType instance.AppType, baseURL, apiKey string, timeout time.Duration) (App, error) {
	cfg, ok := appConfigs[appType]
	if !ok {
		return nil, fmt.Errorf("application type %q is not supported", appType)
	}

	return newAdapter(baseURL, apiKey, timeout, cfg), nil
}
