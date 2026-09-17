package arr

import (
	"context"
	"fmt"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

// *arr history event type IDs, mirroring each application's server-side enum in src/NzbDrone.Core/History.
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

	// Whisparr v3 "Eros" (MovieHistoryEventType): Radarr's enum plus diskScanImported for files found by a disk scan.
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

	fileEndpointEpisode = "episodefile"
	fileEndpointMovie   = "moviefile"
	fileEndpointTrack   = "trackfile"
)

var (
	sonarrHistory = historyConfig{
		deleteEventType:  sonarrFileDeleted,
		importEventTypes: []int{sonarrImported},
		itemIDField:      historyFieldEpisode,
		fileEndpoint:     fileEndpointEpisode,
	}
	radarrHistory = historyConfig{
		deleteEventType:  radarrFileDeleted,
		importEventTypes: []int{radarrImported},
		itemIDField:      historyFieldMovie,
		fileEndpoint:     fileEndpointMovie,
	}
	lidarrHistory = historyConfig{
		deleteEventType:  lidarrFileDeleted,
		importEventTypes: []int{lidarrImported},
		itemIDField:      historyFieldAlbum,
		fileEndpoint:     fileEndpointTrack,
	}
	whisparrV3History = historyConfig{
		deleteEventType:  whisparrV3FileDeleted,
		importEventTypes: []int{whisparrV3Imported, whisparrV3DiskImported},
		itemIDField:      historyFieldMovie,
		fileEndpoint:     fileEndpointMovie,
	}
)

// newHistoryFunc returns a fetchHistoryFunc bound to the given per-app history configuration.
func newHistoryFunc(cfg historyConfig) fetchHistoryFunc {
	return func(ctx context.Context, c *client, apiVersion string,
		since time.Time, pageSize int,
	) ([]HistoryRecord, error) {
		return fetchArrHistory(ctx, c, apiVersion, since, pageSize, cfg)
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
		fetchHistory: newHistoryFunc(sonarrHistory),
	},
	instance.AppTypeRadarr: {
		name:         string(instance.AppTypeRadarr),
		apiVersion:   "v3",
		commandKey:   cmdMoviesSearch,
		idField:      idFieldMovies,
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(radarrHistory),
	},
	instance.AppTypeLidarr: {
		name:         string(instance.AppTypeLidarr),
		apiVersion:   "v1",
		commandKey:   cmdAlbumSearch,
		idField:      idFieldAlbums,
		fetchLibrary: fetchLidarrLibrary,
		fetchHistory: newHistoryFunc(lidarrHistory),
	},
	// Whisparr v2 shares Sonarr's episode-based API; versionMajor tells the two Whisparr generations apart on connect.
	instance.AppTypeWhisparrV2: {
		name:         string(instance.AppTypeWhisparrV2),
		apiVersion:   "v3",
		commandKey:   cmdEpisodeSearch,
		idField:      idFieldEpisodes,
		versionMajor: 2,
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: newHistoryFunc(sonarrHistory),
	},
	// Whisparr v3 "Eros" is Radarr-shaped: scenes are modelled as movies.
	instance.AppTypeWhisparrV3: {
		name:         string(instance.AppTypeWhisparrV3),
		apiVersion:   "v3",
		commandKey:   cmdMoviesSearch,
		idField:      idFieldMovies,
		versionMajor: 3,
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: newHistoryFunc(whisparrV3History),
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
