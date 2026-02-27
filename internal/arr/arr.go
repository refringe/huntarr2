// Package arr provides client implementations for *arr applications
// (Sonarr, Radarr, Lidarr, Whisparr). The shared App interface abstracts
// over application-specific APIs so the scheduler and API handlers can
// operate on any *arr type uniformly.
package arr

import (
	"context"
	"time"
)

// SystemStatus holds the identity and version of a connected *arr instance.
type SystemStatus struct {
	AppName string
	Version string
}

// QualityLevel identifies a single quality tier (e.g. "Bluray-1080p") by
// its numeric ID and human-readable name.
type QualityLevel struct {
	ID   int
	Name string
}

// ProfileEntry is one row in a quality profile's ordered list. It is either
// an individual quality (Quality non-nil) or a named group containing
// nested entries. Groups carry their own ID so the profile's Cutoff field
// can reference either a quality ID or a group ID.
type ProfileEntry struct {
	ID      int
	Quality *QualityLevel
	Name    string
	Items   []ProfileEntry
	Allowed bool
}

// QualityProfile represents a single quality profile configured in an *arr
// application. Items contains the ordered list of qualities and groups;
// the order determines rank (higher index = higher quality).
type QualityProfile struct {
	ID             int
	Name           string
	UpgradeAllowed bool
	Cutoff         int
	Items          []ProfileEntry
}

// LibraryItem represents a single media item (episode, movie, album)
// from an *arr library with enough information to evaluate whether it
// can be upgraded. For items backed by multiple files (Lidarr albums)
// CurrentQualityIDs holds a quality ID per file so that the upgrade
// check can use the lowest ranked track.
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

// FilterStats reports how many items were excluded at each stage of
// upgrade filtering. This aids diagnosis when an instance with a large
// library produces zero upgradeable items.
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

// SearchResult holds the outcome of a search command sent to an *arr
// application.
type SearchResult struct {
	CommandID int
}

// HistoryRecord represents a single import event from an *arr instance's
// history. IsUpgrade is true when the import replaced an existing file
// with a higher quality version. DetailPath holds the path portion of the
// item's detail page in the *arr UI (e.g. "/movie/the-dark-knight-2008"),
// derived from the entity embedded in the history response.
type HistoryRecord struct {
	ID         int
	Date       time.Time
	ItemLabel  string
	DetailPath string
	IsUpgrade  bool
	Quality    string
}

// App is the interface that all *arr application adapters implement. It
// provides a uniform way to query status, quality profiles, library items,
// and to trigger searches regardless of the underlying application type.
type App interface {
	Status(ctx context.Context) (SystemStatus, error)
	QualityProfiles(ctx context.Context) ([]QualityProfile, error)
	LibraryItems(ctx context.Context) ([]LibraryItem, error)
	Search(ctx context.Context, itemIDs []int) (SearchResult, error)
	History(ctx context.Context, since time.Time, pageSize int) ([]HistoryRecord, error)
}
