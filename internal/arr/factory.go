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
		name:         "sonarr",
		apiVersion:   "v3",
		commandKey:   "EpisodeSearch",
		idField:      "episodeIds",
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, sonarrImported, "episodeId"),
	},
	instance.AppTypeRadarr: {
		name:         "radarr",
		apiVersion:   "v3",
		commandKey:   "MoviesSearch",
		idField:      "movieIds",
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(radarrFileDeleted, radarrImported, "movieId"),
	},
	instance.AppTypeLidarr: {
		name:         "lidarr",
		apiVersion:   "v1",
		commandKey:   "AlbumSearch",
		idField:      "albumIds",
		fetchLibrary: fetchLidarrLibrary,
		fetchHistory: newHistoryFunc(lidarrFileDeleted, lidarrImported, "albumId"),
	},
	// Whisparr shares Sonarr's episode-based structure and API.
	instance.AppTypeWhisparr: {
		name:         "whisparr",
		apiVersion:   "v3",
		commandKey:   "EpisodeSearch",
		idField:      "episodeIds",
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, sonarrImported, "episodeId"),
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
