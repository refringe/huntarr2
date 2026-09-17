// Package arr provides clients for *arr applications (Sonarr, Radarr, Lidarr, Whisparr v2/v3) behind the App interface.
package arr

import (
	"context"
	"errors"
	"time"
)

// SystemStatus holds the identity and version of a connected *arr instance.
type SystemStatus struct {
	AppName string
	Version string
}

// QualityLevel identifies a single quality tier (e.g. "Bluray-1080p") by its numeric ID and human-readable name.
type QualityLevel struct {
	ID   int
	Name string
}

// ProfileEntry is one row in a quality profile: a single quality (Quality non-nil) or a named group of nested entries.
type ProfileEntry struct {
	ID      int
	Quality *QualityLevel
	Name    string
	Items   []ProfileEntry
	Allowed bool
}

// QualityProfile is a quality profile whose Items are ordered from lowest to highest quality.
type QualityProfile struct {
	ID             int
	Name           string
	UpgradeAllowed bool
	Cutoff         int
	Items          []ProfileEntry
}

// LibraryItem is a media item (episode, movie, album) with one CurrentQualityIDs entry per backing file.
type LibraryItem struct {
	ID                int
	Label             string
	DetailPath        string
	QualityProfileID  int
	CurrentQualityIDs []int
	HasFile           bool
	Monitored         bool
}

// UpgradeItem identifies a single item eligible for a quality upgrade.
type UpgradeItem struct {
	ID         int
	Label      string
	DetailPath string
}

// FilterStats reports how many items were excluded at each stage of upgrade filtering.
type FilterStats struct {
	LibraryTotal   int
	NoFile         int
	Unmonitored    int
	NoProfile      int
	UpgradeBlocked int
	UnknownQuality int
	AtOrAbove      int
	Upgradeable    int
}

// SearchResult holds the outcome of a search command sent to an *arr application.
type SearchResult struct {
	CommandID int
}

// MediaInfo holds the technical details of an imported file; video fields are empty for Lidarr.
type MediaInfo struct {
	Resolution            string
	VideoCodec            string
	VideoBitDepth         int
	VideoBitrate          int64
	VideoDynamicRangeType string
	AudioCodec            string
	AudioChannels         float64
	AudioBitrate          int64
	AudioBitrateText      string
	AudioBits             string
	AudioSampleRate       string
	RunTime               string
}

// HistoryRecord represents a single import event from an *arr instance's history.
type HistoryRecord struct {
	ID                        int
	ItemID                    int
	Date                      time.Time
	ItemLabel                 string
	ReleaseTitle              string
	DetailPath                string
	MediaCoverPath            string
	IsUpgrade                 bool
	Quality                   string
	PreviousQuality           string
	Size                      int64
	PreviousSize              int64
	CustomFormatScore         *int
	PreviousCustomFormatScore *int
	ReleaseGroup              string
	FileID                    int
	MediaInfo                 *MediaInfo
}

// MediaCover is an image fetched from an *arr instance's mediacover API.
type MediaCover struct {
	ContentType string
	Data        []byte
}

// ErrNotFound indicates the *arr application has no resource at the requested path.
var ErrNotFound = errors.New("resource not found")

// App is the interface that all *arr application adapters implement.
type App interface {
	Status(ctx context.Context) (SystemStatus, error)
	QualityProfiles(ctx context.Context) ([]QualityProfile, error)
	LibraryItems(ctx context.Context) ([]LibraryItem, error)
	Search(ctx context.Context, itemIDs []int) (SearchResult, error)
	History(ctx context.Context, since time.Time, pageSize int) ([]HistoryRecord, error)
	MediaCover(ctx context.Context, path string) (MediaCover, error)
}
