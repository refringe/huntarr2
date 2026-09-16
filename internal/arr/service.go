package arr

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/instance"
)

// ErrVersionMismatch indicates a connection test reached the server, but the reported major version does not match
// the selected application type.
var ErrVersionMismatch = errors.New("application version mismatch")

// InstanceStatus holds the connection status and version for a single *arr instance.
type InstanceStatus struct {
	ID        uuid.UUID
	Name      string
	AppType   instance.AppType
	Connected bool
	Version   string
}

// Service aggregates data from all *arr instances (Sonarr, Radarr, Lidarr, Whisparr v2/v3).
type Service struct {
	instances instance.Repository
	newApp    func(appType instance.AppType, baseURL, apiKey string, timeout time.Duration) (App, error)
}

// NewService returns a Service that reads *arr instances from the given repository.
func NewService(instances instance.Repository) *Service {
	return &Service{instances: instances, newApp: NewApp}
}

// Status fetches connection status for every instance concurrently, marking unreachable instances as disconnected.
func (s *Service) Status(ctx context.Context) ([]InstanceStatus, error) {
	insts, err := s.instances.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instances: %w", err)
	}

	statuses := make([]InstanceStatus, len(insts))
	var wg sync.WaitGroup
	for i, inst := range insts {
		statuses[i] = InstanceStatus{
			ID:      inst.ID,
			Name:    inst.Name,
			AppType: inst.AppType,
		}

		app, err := s.newApp(inst.AppType, inst.BaseURL, inst.APIKey, instanceTimeout(inst))
		if err != nil {
			log.Warn().Err(err).Str("instance", inst.Name).
				Msg("unsupported app type for status check")
			continue
		}

		wg.Go(func() {
			sys, err := app.Status(ctx)
			if err != nil {
				log.Warn().Err(err).Str("instance", inst.Name).
					Msg("arr instance unreachable")
				return
			}
			statuses[i].Connected = true
			statuses[i].Version = sys.Version
		})
	}
	wg.Wait()

	return statuses, nil
}

// TestConnection attempts to reach an *arr instance at the given address, returning nil on success and
// ErrVersionMismatch when the server's major version does not match one the application type demands.
func (s *Service) TestConnection(ctx context.Context, appType instance.AppType, baseURL, apiKey string, timeoutMs int) error {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	app, err := s.newApp(appType, baseURL, apiKey, timeout)
	if err != nil {
		return fmt.Errorf("creating app client: %w", err)
	}
	sys, err := app.Status(ctx)
	if err != nil {
		return fmt.Errorf("testing connection: %w", err)
	}
	if want := appConfigs[appType].versionMajor; want > 0 {
		if got := majorVersion(sys.Version); got > 0 && got != want {
			return fmt.Errorf("%w: the server reports version %s; expected a v%d server for %s",
				ErrVersionMismatch, sys.Version, want, appType)
		}
	}
	return nil
}

// majorVersion returns the leading integer of a dotted version string, or 0 when it cannot be parsed.
func majorVersion(version string) int {
	head, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(head)
	if err != nil || major < 0 {
		return 0
	}
	return major
}

// UpgradeResult holds the items eligible for upgrade, monitored items with no file (missing), and diagnostic
// statistics from the filtering process.
type UpgradeResult struct {
	Items        []UpgradeItem
	MissingItems []UpgradeItem
	Stats        FilterStats
}

// Upgradeable returns all items from the specified instance whose current file quality is below the profile's cutoff.
func (s *Service) Upgradeable(ctx context.Context, instanceID uuid.UUID) (UpgradeResult, error) {
	app, err := s.appForInstance(ctx, instanceID)
	if err != nil {
		return UpgradeResult{}, err
	}
	return s.upgradeableWith(ctx, app)
}

// upgradeableWith fetches an app's quality profiles and library, splitting items into upgradeable and missing sets.
func (s *Service) upgradeableWith(ctx context.Context, app App) (UpgradeResult, error) {
	profiles, err := app.QualityProfiles(ctx)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("fetching quality profiles: %w", err)
	}

	profileMap := make(map[int]QualityProfile, len(profiles))
	for _, p := range profiles {
		profileMap[p.ID] = p
	}

	items, err := app.LibraryItems(ctx)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("fetching library items: %w", err)
	}

	upgradeItems, stats := filterUpgradeable(items, profileMap)
	missingItems := filterMissing(items)
	return UpgradeResult{
		Items:        upgradeItems,
		MissingItems: missingItems,
		Stats:        stats,
	}, nil
}

// Search triggers a search for the given item IDs on the specified instance.
func (s *Service) Search(ctx context.Context, instanceID uuid.UUID, itemIDs []int) (SearchResult, error) {
	app, err := s.appForInstance(ctx, instanceID)
	if err != nil {
		return SearchResult{}, err
	}
	return app.Search(ctx, itemIDs)
}

// SearchCycle fetches all upgradeable and missing items and triggers a search for up to batchSize of them,
// bypassing cooldown filtering and recording. It returns the number of items searched.
func (s *Service) SearchCycle(ctx context.Context, instanceID uuid.UUID, batchSize int) (int, error) {
	app, err := s.appForInstance(ctx, instanceID)
	if err != nil {
		return 0, fmt.Errorf("building app client: %w", err)
	}

	result, err := s.upgradeableWith(ctx, app)
	if err != nil {
		return 0, fmt.Errorf("fetching upgradeable items: %w", err)
	}

	items := make([]UpgradeItem, 0, len(result.Items)+len(result.MissingItems))
	items = append(items, result.Items...)
	items = append(items, result.MissingItems...)
	if len(items) == 0 {
		return 0, nil
	}

	if len(items) > batchSize {
		items = items[:batchSize]
	}

	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}

	if _, err := app.Search(ctx, ids); err != nil {
		return 0, fmt.Errorf("searching items: %w", err)
	}

	return len(ids), nil
}

// History fetches recent import history from the specified instance, returning records dated after since.
func (s *Service) History(ctx context.Context, instanceID uuid.UUID, since time.Time, pageSize int) ([]HistoryRecord, error) {
	app, err := s.appForInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return app.History(ctx, since, pageSize)
}

func (s *Service) appForInstance(ctx context.Context, id uuid.UUID) (App, error) {
	inst, err := s.instances.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching instance %s: %w", id, err)
	}
	app, err := s.newApp(inst.AppType, inst.BaseURL, inst.APIKey, instanceTimeout(inst))
	if err != nil {
		return nil, fmt.Errorf("building app for instance %s: %w", id, err)
	}
	return app, nil
}

func instanceTimeout(inst instance.Instance) time.Duration {
	return time.Duration(inst.TimeoutMs) * time.Millisecond
}
