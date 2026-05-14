package arr

import (
	"context"
	"fmt"
	"time"
)

// cmdNameField is the JSON field naming the command in an *arr command
// request payload.
const cmdNameField = "name"

// fetchLibraryFunc fetches all library items from an *arr instance.
type fetchLibraryFunc func(ctx context.Context, client *client, apiVersion string) ([]LibraryItem, error)

// fetchHistoryFunc fetches recent import history from an *arr instance,
// returning only records dated after since.
type fetchHistoryFunc func(ctx context.Context, client *client,
	apiVersion string, since time.Time, pageSize int) ([]HistoryRecord, error)

// appConfig holds the per-application parameters that distinguish one *arr
// adapter from another.
type appConfig struct {
	name         string
	apiVersion   string
	commandKey   string
	idField      string
	fetchLibrary fetchLibraryFunc
	fetchHistory fetchHistoryFunc
}

// statusResponse is the shared JSON shape returned by all *arr system/status
// endpoints.
type statusResponse struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

// profileEntryResponse mirrors the recursive JSON structure of a quality
// profile entry returned by all *arr qualityprofile endpoints. The
// top-level ID field is populated for group entries and is used by the
// profile's Cutoff to reference a group.
type profileEntryResponse struct {
	ID      int `json:"id"`
	Quality *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"quality"`
	Name    string                 `json:"name"`
	Items   []profileEntryResponse `json:"items"`
	Allowed bool                   `json:"allowed"`
}

// qualityProfileResponse is the shared JSON shape returned by all *arr
// qualityprofile endpoints.
type qualityProfileResponse struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	UpgradeAllowed bool                   `json:"upgradeAllowed"`
	Cutoff         int                    `json:"cutoff"`
	Items          []profileEntryResponse `json:"items"`
}

// commandResponse is the shared JSON shape returned by all *arr command
// endpoints.
type commandResponse struct {
	ID int `json:"id"`
}

// adapter implements App for any *arr application by parameterising the
// differences through appConfig.
type adapter struct {
	client *client
	cfg    appConfig
}

func newAdapter(baseURL, apiKey string, timeout time.Duration, cfg appConfig) App {
	return &adapter{
		client: newClient(baseURL, apiKey, timeout),
		cfg:    cfg,
	}
}

func (a *adapter) Status(ctx context.Context) (SystemStatus, error) {
	var raw statusResponse
	path := fmt.Sprintf("/api/%s/system/status", a.cfg.apiVersion)
	if err := a.client.get(ctx, path, &raw); err != nil {
		return SystemStatus{}, fmt.Errorf("%s system status: %w", a.cfg.name, err)
	}
	return SystemStatus(raw), nil
}

func (a *adapter) QualityProfiles(ctx context.Context) ([]QualityProfile, error) {
	var raw []qualityProfileResponse
	path := fmt.Sprintf("/api/%s/qualityprofile", a.cfg.apiVersion)
	if err := a.client.get(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("%s quality profiles: %w", a.cfg.name, err)
	}

	profiles := make([]QualityProfile, len(raw))
	for i, p := range raw {
		profiles[i] = QualityProfile{
			ID:             p.ID,
			Name:           p.Name,
			UpgradeAllowed: p.UpgradeAllowed,
			Cutoff:         p.Cutoff,
			Items:          convertProfileEntries(p.Items),
		}
	}
	return profiles, nil
}

// convertProfileEntries converts the JSON response entries into domain
// ProfileEntry values.
func convertProfileEntries(raw []profileEntryResponse) []ProfileEntry {
	entries := make([]ProfileEntry, len(raw))
	for i, r := range raw {
		entries[i] = ProfileEntry{
			ID:      r.ID,
			Name:    r.Name,
			Allowed: r.Allowed,
			Items:   convertProfileEntries(r.Items),
		}
		if r.Quality != nil && r.Quality.ID != 0 {
			entries[i].Quality = &QualityLevel{
				ID:   r.Quality.ID,
				Name: r.Quality.Name,
			}
		}
	}
	return entries
}

func (a *adapter) LibraryItems(ctx context.Context) ([]LibraryItem, error) {
	return a.cfg.fetchLibrary(ctx, a.client, a.cfg.apiVersion)
}

func (a *adapter) History(ctx context.Context, since time.Time, pageSize int) ([]HistoryRecord, error) {
	return a.cfg.fetchHistory(ctx, a.client, a.cfg.apiVersion, since, pageSize)
}

func (a *adapter) Search(ctx context.Context, itemIDs []int) (SearchResult, error) {
	if len(itemIDs) == 0 {
		return SearchResult{}, fmt.Errorf("%s search: no item IDs provided", a.cfg.name)
	}

	cmd := map[string]any{
		cmdNameField:  a.cfg.commandKey,
		a.cfg.idField: itemIDs,
	}
	var raw commandResponse
	path := fmt.Sprintf("/api/%s/command", a.cfg.apiVersion)
	if err := a.client.post(ctx, path, cmd, &raw); err != nil {
		return SearchResult{}, fmt.Errorf("%s search: %w", a.cfg.name, err)
	}

	return SearchResult{CommandID: raw.ID}, nil
}
