// Package pages provides templ page components and their view models for the Huntarr2 web interface.
package pages

import "time"

// HomeData holds the data rendered on the Home page.
type HomeData struct {
	AssetVersion     string
	HasArrInstances  bool
	AllTimeSearches  int
	AllTimeSkipped   int
	AllTimeUpgrades  int
	AllTimeDownloads int
	RecentSearches   int
	RecentSkipped    int
	RecentUpgrades   int
	RecentDownloads  int
	PerInstance      []HomeInstanceStats
	LatestUpgrades   []HomeUpgrade
	SchedulerRunning bool
	SearchesThisHour int
	HourlyLimit      int
	ArrInstances     []HomeArrInstance
}

// HomeInstanceStats holds per-instance search/skip/upgrade/download counts for the dashboard.
type HomeInstanceStats struct {
	InstanceID    string
	InstanceName  string
	AppType       string
	SearchCount   int
	SkipCount     int
	UpgradeCount  int
	DownloadCount int
}

// HomeUpgrade is one row of the latest upgrades list on the Home page.
type HomeUpgrade struct {
	InstanceName string
	AppType      string
	ItemLabel    string
	ReleaseTitle string
	DetailURL    string
	PosterURL    string
	FromQuality  string
	ToQuality    string
	FromSize     int64
	ToSize       int64
	FromScore    *int
	ToScore      *int
	MediaTags    []string
	DetectedAt   time.Time
}

// HomeArrInstance represents a single *arr instance status on the Home page.
type HomeArrInstance struct {
	Name      string
	AppType   string
	Connected bool
	Version   string
}

// ConnectionsData holds the data rendered on the Connections page.
type ConnectionsData struct {
	AssetVersion string
	Instances    []ConnectionInstance
}

// ConnectionInstance represents a single instance card on the Connections page.
type ConnectionInstance struct {
	ID        string
	Name      string
	AppType   string
	BaseURL   string
	HasAPIKey bool
}

// LogsData holds the data rendered on the Logs page.
type LogsData struct {
	AssetVersion string
	Instances    []LogsInstance
}

// LogsInstance represents an instance available for filtering on the Logs page.
type LogsInstance struct {
	ID   string
	Name string
}

// SettingsData holds the data rendered on the Settings page.
type SettingsData struct {
	AssetVersion string
	Instances    []SettingsInstance
}

// SettingsInstance represents an *arr instance on the Settings page.
type SettingsInstance struct {
	ID      string
	Name    string
	AppType string
}
