package arr

import (
	"fmt"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

// appConfigs maps each supported application type to its adapter
// configuration. The map is read-only after initialisation.
var appConfigs = map[instance.AppType]appConfig{
	instance.AppTypeSonarr: {
		name:         "sonarr",
		apiVersion:   "v3",
		commandKey:   "EpisodeSearch",
		idField:      "episodeIds",
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: fetchArrHistory,
	},
	instance.AppTypeRadarr: {
		name:         "radarr",
		apiVersion:   "v3",
		commandKey:   "MoviesSearch",
		idField:      "movieIds",
		fetchLibrary: fetchRadarrLibrary,
		fetchHistory: fetchArrHistory,
	},
	instance.AppTypeLidarr: {
		name:         "lidarr",
		apiVersion:   "v1",
		commandKey:   "AlbumSearch",
		idField:      "albumIds",
		fetchLibrary: fetchLidarrLibrary,
		fetchHistory: fetchArrHistory,
	},
	// Whisparr shares Sonarr's episode-based structure and API.
	instance.AppTypeWhisparr: {
		name:         "whisparr",
		apiVersion:   "v3",
		commandKey:   "EpisodeSearch",
		idField:      "episodeIds",
		fetchLibrary: fetchSonarrLibrary,
		fetchHistory: fetchArrHistory,
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
