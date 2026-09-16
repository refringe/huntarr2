package arr

import (
	"context"
	"fmt"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

// *arr history event type integer IDs. Each application defines its own enum; the values below are extracted from
// the respective source repositories (src/NzbDrone.Core/History) and must match the server-side definitions.
const (
	// Sonarr / Whisparr v2 (EpisodeHistoryEventType).
	sonarrImported    = 3 // downloadFolderImported
	sonarrFileDeleted = 5 // episodeFileDeleted

	// Radarr (MovieHistoryEventType): note 2 is a deprecated slot and 4 is downloadFailed.
	radarrImported    = 3 // downloadFolderImported
	radarrFileDeleted = 6 // movieFileDeleted

	// Lidarr (EntityHistoryEventType): note 4 is downloadFailed.
	lidarrImported    = 3 // trackFileImported
	lidarrFileDeleted = 5 // trackFileDeleted

	// Whisparr v3 "Eros" (MovieHistoryEventType): Radarr's enum plus diskScanImported, a second import-completed
	// event written when a disk scan finds a new in-place file.
	whisparrV3Imported     = 3  // downloadFolderImported
	whisparrV3DiskImported = 10 // diskScanImported
	whisparrV3FileDeleted  = 6  // movieFileDeleted
)

// Per-application command names, search ID field names, and history item ID field names referenced by appConfigs.
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

// newHistoryFunc returns a fetchHistoryFunc bound to the per-app event type IDs and item ID field.
func newHistoryFunc(deleteEventType int, importEventTypes []int, itemIDField string) fetchHistoryFunc {
	return func(ctx context.Context, c *client, apiVersion string,
		since time.Time, pageSize int,
	) ([]HistoryRecord, error) {
		return fetchArrHistory(ctx, c, apiVersion, since, pageSize,
			deleteEventType, importEventTypes, itemIDField)
	}
}

// appConfigs maps each supported application type to its read-only adapter configuration.
var appConfigs = map[instance.AppType]appConfig{
	instance.AppTypeSonarr: {
		name:         string(instance.AppTypeSonarr),
		apiVersion:   "v3",
		commandKey:   cmdEpisodeSearch,
		idField:      idFieldEpisodes,
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, []int{sonarrImported}, historyFieldEpisode),
	},
	instance.AppTypeRadarr: {
		name:         string(instance.AppTypeRadarr),
		apiVersion:   "v3",
		commandKey:   cmdMoviesSearch,
		idField:      idFieldMovies,
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(radarrFileDeleted, []int{radarrImported}, historyFieldMovie),
	},
	instance.AppTypeLidarr: {
		name:         string(instance.AppTypeLidarr),
		apiVersion:   "v1",
		commandKey:   cmdAlbumSearch,
		idField:      idFieldAlbums,
		fetchLibrary: fetchLidarrLibrary,
		fetchHistory: newHistoryFunc(lidarrFileDeleted, []int{lidarrImported}, historyFieldAlbum),
	},
	// Whisparr v2 shares Sonarr's episode-based structure and API. Both Whisparr generations report appName
	// "Whisparr" on system/status; versionMajor tells a connection test which generation it reached.
	instance.AppTypeWhisparrV2: {
		name:         string(instance.AppTypeWhisparrV2),
		apiVersion:   "v3",
		commandKey:   cmdEpisodeSearch,
		idField:      idFieldEpisodes,
		versionMajor: 2,
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrFileDeleted, []int{sonarrImported}, historyFieldEpisode),
	},
	// Whisparr v3 "Eros" is Radarr-shaped: scenes are modelled as movies.
	instance.AppTypeWhisparrV3: {
		name:         string(instance.AppTypeWhisparrV3),
		apiVersion:   "v3",
		commandKey:   cmdMoviesSearch,
		idField:      idFieldMovies,
		versionMajor: 3,
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(whisparrV3FileDeleted,
			[]int{whisparrV3Imported, whisparrV3DiskImported}, historyFieldMovie),
	},
}

// NewApp constructs the appropriate App implementation for the given application type.
func NewApp(appType instance.AppType, baseURL, apiKey string, timeout time.Duration) (App, error) {
	cfg, ok := appConfigs[appType]
	if !ok {
		return nil, fmt.Errorf("application type %q is not supported", appType)
	}

	return newAdapter(baseURL, apiKey, timeout, cfg), nil
}
