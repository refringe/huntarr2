package scheduler

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/internal/settings"
)

// fakeSettingsResolver returns preconfigured resolved settings.
type fakeSettingsResolver struct {
	global  settings.Resolved
	perInst map[uuid.UUID]settings.Resolved
}

func newFakeSettingsResolver() *fakeSettingsResolver {
	return &fakeSettingsResolver{
		global:  settings.Defaults(),
		perInst: make(map[uuid.UUID]settings.Resolved),
	}
}

func (f *fakeSettingsResolver) ResolveGlobal(_ context.Context) (settings.Resolved, error) {
	return f.global, nil
}

func (f *fakeSettingsResolver) Resolve(_ context.Context, id uuid.UUID) (settings.Resolved, error) {
	if r, ok := f.perInst[id]; ok {
		return r, nil
	}
	return f.global, nil
}

// fakeCooldownTracker tracks cooldown state in memory.
type fakeCooldownTracker struct {
	mu               sync.Mutex
	coolingDown      map[uuid.UUID]map[int]struct{}
	recorded         map[uuid.UUID][]int
	deletedOlderThan []time.Duration
}

func newFakeCooldownTracker() *fakeCooldownTracker {
	return &fakeCooldownTracker{
		coolingDown: make(map[uuid.UUID]map[int]struct{}),
		recorded:    make(map[uuid.UUID][]int),
	}
}

func (f *fakeCooldownTracker) FilterCoolingDown(
	_ context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
	_ time.Duration,
) ([]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cooling := f.coolingDown[instanceID]
	var result []int
	for _, id := range itemIDs {
		if _, ok := cooling[id]; ok {
			result = append(result, id)
		}
	}
	return result, nil
}

func (f *fakeCooldownTracker) RecordSearches(
	_ context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded[instanceID] = append(f.recorded[instanceID], itemIDs...)
	return nil
}

func (f *fakeCooldownTracker) DeleteExpired(
	_ context.Context,
	olderThan time.Duration,
) (int64, error) {
	f.deletedOlderThan = append(f.deletedOlderThan, olderThan)
	return 0, nil
}

// fakeActivityLogger collects logged activity entries.
type fakeActivityLogger struct {
	mu      sync.Mutex
	entries []activity.Entry
}

func (f *fakeActivityLogger) Log(_ context.Context, entry *activity.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry.ID = uuid.New()
	entry.CreatedAt = time.Now()
	f.entries = append(f.entries, *entry)
	return nil
}

func (f *fakeActivityLogger) Prune(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

// fakeArrSearcher records calls and returns preconfigured results.
type fakeArrSearcher struct {
	mu               sync.Mutex
	upgradeableItems map[uuid.UUID][]arr.UpgradeItem
	missingItems     map[uuid.UUID][]arr.UpgradeItem
	historyRecords   map[uuid.UUID][]arr.HistoryRecord
	searchCalls      []searchCall
	searchErr        error

	// upgradeableDelay makes Upgradeable sleep before returning; upgradeableReturnedAt records when it returned.
	upgradeableDelay      time.Duration
	upgradeableReturnedAt time.Time

	// instanceDelays makes Upgradeable sleep per instance; upgradeableCalls counts its calls per instance.
	instanceDelays   map[uuid.UUID]time.Duration
	upgradeableCalls map[uuid.UUID]int
}

type searchCall struct {
	instanceID uuid.UUID
	itemIDs    []int
}

func newFakeArrSearcher() *fakeArrSearcher {
	return &fakeArrSearcher{
		upgradeableItems: make(map[uuid.UUID][]arr.UpgradeItem),
		missingItems:     make(map[uuid.UUID][]arr.UpgradeItem),
		historyRecords:   make(map[uuid.UUID][]arr.HistoryRecord),
		instanceDelays:   make(map[uuid.UUID]time.Duration),
		upgradeableCalls: make(map[uuid.UUID]int),
	}
}

func (f *fakeArrSearcher) Upgradeable(
	_ context.Context,
	instanceID uuid.UUID,
) (arr.UpgradeResult, error) {
	if delay := f.upgradeableDelay + f.instanceDelays[instanceID]; delay > 0 {
		time.Sleep(delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upgradeableCalls[instanceID]++
	items := f.upgradeableItems[instanceID]
	missing := f.missingItems[instanceID]
	f.upgradeableReturnedAt = time.Now()
	return arr.UpgradeResult{
		Items:        items,
		MissingItems: missing,
		Stats:        arr.FilterStats{Upgradeable: len(items)},
	}, nil
}

func (f *fakeArrSearcher) Search(
	_ context.Context,
	instanceID uuid.UUID,
	itemIDs []int,
) (arr.SearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searchCalls = append(f.searchCalls, searchCall{instanceID, itemIDs})
	return arr.SearchResult{CommandID: 1}, f.searchErr
}

func (f *fakeArrSearcher) History(
	_ context.Context,
	instanceID uuid.UUID,
	_ time.Time,
	_ int,
) ([]arr.HistoryRecord, error) {
	return f.historyRecords[instanceID], nil
}

// fakeInstanceLister returns a static list of instances.
type fakeInstanceLister struct {
	instances []instance.Instance
}

func (f *fakeInstanceLister) List(_ context.Context) ([]instance.Instance, error) {
	return f.instances, nil
}

func (f *fakeInstanceLister) Get(_ context.Context, id uuid.UUID) (instance.Instance, error) {
	for _, inst := range f.instances {
		if inst.ID == id {
			return inst, nil
		}
	}
	return instance.Instance{}, instance.ErrNotFound
}

// fakePollTracker records poll calls and returns preconfigured timestamps.
type fakePollTracker struct {
	lastPolled map[uuid.UUID]time.Time
	pollCalls  []uuid.UUID
}

func newFakePollTracker() *fakePollTracker {
	return &fakePollTracker{
		lastPolled: make(map[uuid.UUID]time.Time),
	}
}

func (f *fakePollTracker) LastPolled(
	_ context.Context,
	instanceID uuid.UUID,
) (time.Time, error) {
	return f.lastPolled[instanceID], nil
}

func (f *fakePollTracker) RecordPoll(
	_ context.Context,
	instanceID uuid.UUID,
	polledAt time.Time,
) error {
	f.lastPolled[instanceID] = polledAt
	f.pollCalls = append(f.pollCalls, instanceID)
	return nil
}

func newTestScheduler(
	t *testing.T,
	instances *fakeInstanceLister,
	settingsRes *fakeSettingsResolver,
	cooldowns *fakeCooldownTracker,
	actLog *fakeActivityLogger,
	arrSearch *fakeArrSearcher,
	polls *fakePollTracker,
) *Scheduler {
	t.Helper()
	s, err := New(
		50*time.Millisecond,
		instances,
		settingsRes,
		cooldowns,
		actLog,
		arrSearch,
		polls,
	)
	if err != nil {
		t.Fatalf("creating test scheduler: %v", err)
	}
	return s
}

// runBudget is the fake-clock time each test lets the scheduler loop run for: four 50ms ticks.
const runBudget = 200 * time.Millisecond

// runScheduler drives sched.Run inside a synctest bubble, cancelling the run after runBudget of fake-clock time.
func runScheduler(t *testing.T, sched *Scheduler) error {
	t.Helper()
	var err error
	synctest.Test(t, func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), runBudget)
		defer cancel()
		err = sched.Run(ctx)
	})
	return err
}

func TestStopsOnContextCancellation(t *testing.T) {
	sched := newTestScheduler(t,
		&fakeInstanceLister{},
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		newFakeArrSearcher(),
		newFakePollTracker(),
	)

	err := runScheduler(t, sched)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestSkipsDisabledInstances(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	disabled := settings.Defaults()
	disabled.Enabled = false
	settingsRes.perInst[instID] = disabled

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{
		{ID: 1, Label: "Item 1"},
		{ID: 2, Label: "Item 2"},
	}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) != 0 {
		t.Errorf("searchCalls = %d, want 0 (disabled)", len(arrSearch.searchCalls))
	}
}

func TestFiltersCooldownItems(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{
		{ID: 1, Label: "Item 1"},
		{ID: 2, Label: "Item 2"},
		{ID: 3, Label: "Item 3"},
	}

	cooldowns := newFakeCooldownTracker()
	cooldowns.coolingDown[instID] = map[int]struct{}{1: {}, 3: {}}

	actLog := &fakeActivityLogger{}
	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		cooldowns,
		actLog,
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) == 0 {
		t.Fatal("expected at least one search call")
	}
	call := arrSearch.searchCalls[0]
	if len(call.itemIDs) != 1 || call.itemIDs[0] != 2 {
		t.Errorf("searched items = %v, want [2]", call.itemIDs)
	}

	if len(cooldowns.recorded[instID]) == 0 {
		t.Error("expected cooldown records")
	}

	var cycleEntry *activity.Entry
	for i := range actLog.entries {
		if actLog.entries[i].Action == activity.ActionSearchCycle &&
			actLog.entries[i].Level == activity.LevelInfo {
			cycleEntry = &actLog.entries[i]
			break
		}
	}
	if cycleEntry == nil {
		t.Fatal("no search_cycle info entry logged")
	}
	raw, ok := cycleEntry.Details["searchedItems"]
	if !ok {
		t.Fatal("searchedItems missing from details")
	}
	items, ok := raw.([]map[string]any)
	if !ok {
		t.Fatalf("searchedItems type = %T, want []map[string]any", raw)
	}
	if len(items) != 1 {
		t.Fatalf("searchedItems length = %d, want 1", len(items))
	}
	if items[0]["label"] != "Item 2" {
		t.Errorf("searchedItems[0].label = %v, want Item 2", items[0]["label"])
	}
}

func TestRespectsRateLimit(t *testing.T) {
	inst1 := uuid.New()
	inst2 := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: inst1, Name: "Sonarr", AppType: instance.AppTypeSonarr},
			{ID: inst2, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.SearchLimit = 1

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[inst1] = []arr.UpgradeItem{{ID: 1, Label: "Item 1"}}
	arrSearch.upgradeableItems[inst2] = []arr.UpgradeItem{{ID: 2, Label: "Item 2"}}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) > 1 {
		t.Errorf("searchCalls = %d, want <= 1 (rate limited)", len(arrSearch.searchCalls))
	}
}

// countActions returns the number of logged entries with the given action.
func countActions(actLog *fakeActivityLogger, action activity.Action) int {
	count := 0
	for _, e := range actLog.entries {
		if e.Action == action {
			count++
		}
	}
	return count
}

func TestRateLimitLoggedOncePerHour(t *testing.T) {
	inst1 := uuid.New()
	inst2 := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: inst1, Name: "Sonarr", AppType: instance.AppTypeSonarr},
			{ID: inst2, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.SearchLimit = 1

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[inst1] = []arr.UpgradeItem{{ID: 1}}
	arrSearch.upgradeableItems[inst2] = []arr.UpgradeItem{{ID: 2}}

	actLog := &fakeActivityLogger{}
	sched := newTestScheduler(t,
		instances, settingsRes, newFakeCooldownTracker(), actLog, arrSearch, newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if got := countActions(actLog, activity.ActionRateLimit); got != 1 {
		t.Errorf("rate_limit entries = %d, want 1 across several ticks", got)
	}
	if len(arrSearch.searchCalls) != 1 {
		t.Errorf("searchCalls = %d, want 1 under a limit of 1", len(arrSearch.searchCalls))
	}
}

func TestWindowSkipLoggedOncePerClosedWindow(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	actLog := &fakeActivityLogger{}
	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}}

	synctest.Test(t, func(t *testing.T) {
		// The window opens one hour after the bubble's start time and stays open for an hour.
		start := time.Now().Add(time.Hour)
		settingsRes := newFakeSettingsResolver()
		settingsRes.global.SearchWindowStart = start.Format("15:04")
		settingsRes.global.SearchWindowEnd = start.Add(time.Hour).Format("15:04")

		sched := newTestScheduler(t,
			instances, settingsRes, newFakeCooldownTracker(), actLog, arrSearch, newFakePollTracker(),
		)

		ctx, cancel := context.WithTimeout(t.Context(), runBudget)
		defer cancel()
		_ = sched.Run(ctx)
	})

	if got := countActions(actLog, activity.ActionSearchSkip); got != 1 {
		t.Errorf("search_skip entries = %d, want 1 across several ticks", got)
	}
	if len(arrSearch.searchCalls) != 0 {
		t.Errorf("searchCalls = %d, want 0 outside the window", len(arrSearch.searchCalls))
	}
}

func TestSlowInstanceDoesNotDelayOthers(t *testing.T) {
	slow := uuid.New()
	fast := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: slow, Name: "Slow", AppType: instance.AppTypeSonarr},
			{ID: fast, Name: "Fast", AppType: instance.AppTypeRadarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[slow] = []arr.UpgradeItem{{ID: 1}}
	arrSearch.upgradeableItems[fast] = []arr.UpgradeItem{{ID: 2}}
	arrSearch.instanceDelays[slow] = 120 * time.Millisecond

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		arrSearch, newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) != 2 {
		t.Fatalf("searchCalls = %d, want 2", len(arrSearch.searchCalls))
	}
	if got := arrSearch.searchCalls[0].instanceID; got != fast {
		t.Errorf("first search ran for %s, want the fast instance while the slow one was still fetching", got)
	}
}

func TestRunningCycleIsNotStartedAgain(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}}
	arrSearch.instanceDelays[instID] = 120 * time.Millisecond

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		arrSearch, newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if got := arrSearch.upgradeableCalls[instID]; got != 1 {
		t.Errorf("Upgradeable calls = %d, want 1 while the cycle spans several ticks", got)
	}
}

func TestRunWaitsForRunningCycles(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.instanceDelays[instID] = time.Second

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		arrSearch, newFakePollTracker(),
	)

	var elapsed time.Duration
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		ctx, cancel := context.WithTimeout(t.Context(), runBudget)
		defer cancel()
		_ = sched.Run(ctx)
		elapsed = time.Since(start)
	})

	if elapsed < time.Second {
		t.Errorf("Run returned after %v, want it to wait for the cycle still running", elapsed)
	}
	if len(sched.Status().Instances) != 1 {
		t.Errorf("schedules = %d, want the finished cycle's schedule recorded", len(sched.Status().Instances))
	}
}

func TestSearchNowHonoursCooldownsAndRecordsThem(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}, {ID: 2}, {ID: 3}}

	cooldowns := newFakeCooldownTracker()
	cooldowns.coolingDown[instID] = map[int]struct{}{1: {}}

	actLog := &fakeActivityLogger{}
	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), cooldowns, actLog, arrSearch, newFakePollTracker(),
	)

	searched, err := sched.SearchNow(t.Context(), instID, 50)
	if err != nil {
		t.Fatalf("SearchNow: %v", err)
	}

	if searched != 2 {
		t.Errorf("searched = %d, want 2 (one item is cooling down)", searched)
	}
	if got := cooldowns.recorded[instID]; !slices.Equal(got, []int{2, 3}) {
		t.Errorf("recorded cooldowns = %v, want [2 3]", got)
	}
	if got := sched.Status().SearchesThisHour; got != 2 {
		t.Errorf("SearchesThisHour = %d, want 2", got)
	}
	if got := countActions(actLog, activity.ActionSearchCycle); got != 1 {
		t.Errorf("search_cycle entries = %d, want 1", got)
	}
}

func TestSearchNowStopsAtHourlyLimit(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.SearchLimit = 2

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}, {ID: 2}, {ID: 3}}

	sched := newTestScheduler(t,
		instances, settingsRes, newFakeCooldownTracker(), &fakeActivityLogger{}, arrSearch, newFakePollTracker(),
	)

	searched, err := sched.SearchNow(t.Context(), instID, 50)
	if err != nil {
		t.Fatalf("first SearchNow: %v", err)
	}
	if searched != 2 {
		t.Errorf("searched = %d, want 2 (the hourly limit)", searched)
	}

	if _, err := sched.SearchNow(t.Context(), instID, 50); !errors.Is(err, ErrSearchLimitReached) {
		t.Errorf("second SearchNow err = %v, want ErrSearchLimitReached", err)
	}
}

func TestSearchNowRejectsRunningInstance(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}}
	arrSearch.instanceDelays[instID] = time.Minute

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		arrSearch, newFakePollTracker(),
	)

	synctest.Test(t, func(t *testing.T) {
		first := make(chan error, 1)
		go func() {
			_, err := sched.SearchNow(t.Context(), instID, 50)
			first <- err
		}()
		synctest.Wait()

		if _, err := sched.SearchNow(t.Context(), instID, 50); !errors.Is(err, ErrCycleRunning) {
			t.Errorf("overlapping SearchNow err = %v, want ErrCycleRunning", err)
		}
		if err := <-first; err != nil {
			t.Errorf("first SearchNow: %v", err)
		}
	})
}

func TestSearchNowUnknownInstance(t *testing.T) {
	sched := newTestScheduler(t,
		&fakeInstanceLister{}, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		newFakeArrSearcher(), newFakePollTracker(),
	)

	if _, err := sched.SearchNow(t.Context(), uuid.New(), 50); !errors.Is(err, instance.ErrNotFound) {
		t.Errorf("err = %v, want instance.ErrNotFound", err)
	}
}

func TestSearchNowReturnsCycleError(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}}
	arrSearch.searchErr = errors.New("connection refused")

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		arrSearch, newFakePollTracker(),
	)

	if _, err := sched.SearchNow(t.Context(), instID, 50); err == nil {
		t.Error("err = nil, want the search failure")
	}
	if got := sched.Status().SearchesThisHour; got != 0 {
		t.Errorf("SearchesThisHour = %d, want 0 after a failed search", got)
	}
}

func TestBatchCappedAtRemainingHourlyAllowance(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr}},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.SearchLimit = 3
	settingsRes.global.BatchSize = 5

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) != 1 {
		t.Fatalf("searchCalls = %d, want 1", len(arrSearch.searchCalls))
	}
	if got := len(arrSearch.searchCalls[0].itemIDs); got != 3 {
		t.Errorf("searched %d items, want 3 (the hourly limit)", got)
	}
	if got := sched.Status().SearchesThisHour; got != 3 {
		t.Errorf("SearchesThisHour = %d, want 3", got)
	}
}

func TestHappyPathEndToEnd(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{
		{ID: 1, Label: "Item 1"},
		{ID: 2, Label: "Item 2"},
		{ID: 3, Label: "Item 3"},
	}

	cooldowns := newFakeCooldownTracker()
	actLog := &fakeActivityLogger{}

	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		cooldowns,
		actLog,
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) == 0 {
		t.Fatal("expected at least one search call")
	}

	call := arrSearch.searchCalls[0]
	if len(call.itemIDs) != 3 {
		t.Errorf("searched items = %d, want 3", len(call.itemIDs))
	}

	if len(cooldowns.recorded[instID]) != 3 {
		t.Errorf("recorded cooldowns = %d, want 3", len(cooldowns.recorded[instID]))
	}

	var cycleEntry *activity.Entry
	for i := range actLog.entries {
		if actLog.entries[i].Action == activity.ActionSearchCycle &&
			actLog.entries[i].Level == activity.LevelInfo {
			cycleEntry = &actLog.entries[i]
			break
		}
	}
	if cycleEntry == nil {
		t.Fatal("no search_cycle info entry logged")
	}
	raw, ok := cycleEntry.Details["searchedItems"]
	if !ok {
		t.Fatal("searchedItems missing from details")
	}
	items, ok := raw.([]map[string]any)
	if !ok {
		t.Fatalf("searchedItems type = %T, want []map[string]any", raw)
	}
	if len(items) != 3 {
		t.Errorf("searchedItems length = %d, want 3", len(items))
	}
	wantLabels := map[string]struct{}{
		"Item 1": {},
		"Item 2": {},
		"Item 3": {},
	}
	for _, item := range items {
		label, _ := item["label"].(string)
		if _, exists := wantLabels[label]; !exists {
			t.Errorf("unexpected label %q in searchedItems", label)
		}
	}
}

func TestBatchSizeLimitsSearch(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.BatchSize = 2

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{
		{ID: 1, Label: "Item 1"},
		{ID: 2, Label: "Item 2"},
		{ID: 3, Label: "Item 3"},
		{ID: 4, Label: "Item 4"},
		{ID: 5, Label: "Item 5"},
	}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) == 0 {
		t.Fatal("expected at least one search call")
	}

	call := arrSearch.searchCalls[0]
	if len(call.itemIDs) != 2 {
		t.Errorf("searched items = %d, want 2 (batch size limit)", len(call.itemIDs))
	}
}

func TestStatusSnapshot(t *testing.T) {
	sched := newTestScheduler(t,
		&fakeInstanceLister{},
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		newFakeArrSearcher(),
		newFakePollTracker(),
	)

	st := sched.Status()
	if st.Running {
		t.Error("Running = true before Run(), want false")
	}
}

func TestComputeNextInterval(t *testing.T) {
	base := 6 * time.Hour
	tests := []struct {
		name       string
		previous   time.Duration
		totalItems int
		batchSize  int
		want       time.Duration
	}{
		{"no items from base", base, 0, 10, 12 * time.Hour},
		{"no items progressive", 12 * time.Hour, 0, 10, 24 * time.Hour},
		{"no items capped at 4x", 24 * time.Hour, 0, 10, 24 * time.Hour},
		{"light load", base, 5, 10, base},
		{"equal to batch", base, 10, 10, base},
		{"heavy load from base", base, 25, 10, 3 * time.Hour},
		{"heavy load progressive", 3 * time.Hour, 25, 10, 90 * time.Minute},
		{"heavy load floored at base/4", 90 * time.Minute, 25, 10, 90 * time.Minute},
		{"moderate load", base, 15, 10, base},
		{"normal resets from backoff", 12 * time.Hour, 5, 10, base},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNextInterval(base, tt.previous, tt.totalItems, tt.batchSize)
			if got != tt.want {
				t.Errorf("computeNextInterval(%v, %v, %d, %d) = %v, want %v",
					base, tt.previous, tt.totalItems, tt.batchSize, got, tt.want)
			}
		})
	}
}

func TestInSearchWindow(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		now   time.Time
		want  bool
	}{
		{
			name: "both empty allows always",
			want: true,
		},
		{
			name:  "same day inside window",
			start: "01:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 3, 0, 0, 0, time.UTC),
			want:  true,
		},
		{
			name:  "same day outside window",
			start: "01:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 7, 0, 0, 0, time.UTC),
			want:  false,
		},
		{
			name:  "same day at start boundary",
			start: "01:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
			want:  true,
		},
		{
			name:  "same day at end boundary",
			start: "01:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 6, 0, 0, 0, time.UTC),
			want:  false,
		},
		{
			name:  "cross midnight evening",
			start: "22:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC),
			want:  true,
		},
		{
			name:  "cross midnight morning",
			start: "22:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 3, 0, 0, 0, time.UTC),
			want:  true,
		},
		{
			name:  "cross midnight outside",
			start: "22:00",
			end:   "06:00",
			now:   time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inSearchWindow(tt.start, tt.end, tt.now)
			if got != tt.want {
				t.Errorf("inSearchWindow(%q, %q, %v) = %v, want %v",
					tt.start, tt.end, tt.now, got, tt.want)
			}
		})
	}
}

func TestSearchWindowInvalidInput(t *testing.T) {
	got := inSearchWindow("bad", "also-bad", time.Now())
	if !got {
		t.Error("invalid window input should default to allowing search")
	}
}

func TestExcludeIDs(t *testing.T) {
	all := []int{1, 2, 3, 4, 5}
	excluded := []int{2, 4}
	got := excludeIDs(all, excluded)

	want := map[int]struct{}{1: {}, 3: {}, 5: {}}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for _, id := range got {
		if _, ok := want[id]; !ok {
			t.Errorf("unexpected ID %d in result", id)
		}
	}
}

func TestStatusPopulatesInstanceDetails(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1, Label: "Item 1"}}

	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	st := sched.Status()
	if len(st.Instances) != 1 {
		t.Fatalf("Instances = %d, want 1", len(st.Instances))
	}
	inst := st.Instances[0]
	if inst.InstanceName != "Sonarr" {
		t.Errorf("InstanceName = %q, want %q", inst.InstanceName, "Sonarr")
	}
	if !inst.Enabled {
		t.Error("Enabled = false, want true")
	}
	if inst.NextSearchAt.IsZero() {
		t.Error("NextSearchAt is zero, want non-zero")
	}
}

func TestStatusSortedByInstanceName(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: id1, Name: "Whisparr", AppType: instance.AppTypeSonarr},
			{ID: id2, Name: "Sonarr", AppType: instance.AppTypeSonarr},
			{ID: id3, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	for _, id := range []uuid.UUID{id1, id2, id3} {
		arrSearch.upgradeableItems[id] = []arr.UpgradeItem{{ID: 1, Label: "Item 1"}}
	}

	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	st := sched.Status()
	if len(st.Instances) != 3 {
		t.Fatalf("Instances = %d, want 3", len(st.Instances))
	}

	for i := 1; i < len(st.Instances); i++ {
		prev := st.Instances[i-1].InstanceName
		curr := st.Instances[i].InstanceName
		if prev >= curr {
			t.Errorf("instances not sorted by name: %q >= %q at index %d",
				prev, curr, i)
		}
	}
}

func TestStatusShowsDisabledInstances(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	disabled := settings.Defaults()
	disabled.Enabled = false
	settingsRes.perInst[instID] = disabled

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		newFakeArrSearcher(),
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	st := sched.Status()
	if len(st.Instances) != 1 {
		t.Fatalf("Instances = %d, want 1", len(st.Instances))
	}
	if st.Instances[0].Enabled {
		t.Error("Enabled = true, want false for disabled instance")
	}
	if st.Instances[0].InstanceName != "Sonarr" {
		t.Errorf("InstanceName = %q, want %q",
			st.Instances[0].InstanceName, "Sonarr")
	}
}

func TestNewRejectsZeroTickInterval(t *testing.T) {
	_, err := New(
		0,
		&fakeInstanceLister{},
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		newFakeArrSearcher(),
		newFakePollTracker(),
	)
	if err == nil {
		t.Error("New(0, ...) should return an error")
	}
}

func TestParseHHMMRejectsOutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"hour too high", "25:00"},
		{"minute too high", "12:99"},
		{"negative hour", "-1:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseHHMM(tt.input)
			if err == nil {
				t.Errorf("parseHHMM(%q) = nil, want error", tt.input)
			}
		})
	}
}

func TestPollUpgradeHistoryDetectsUpgrades(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	score, previousScore := 1100, 450
	arrSearch := newFakeArrSearcher()
	arrSearch.historyRecords[instID] = []arr.HistoryRecord{
		{
			ID: 1, ItemLabel: "Movie.Upgraded", DetailPath: "/movie/movie-upgraded", IsUpgrade: true,
			ReleaseTitle: "Movie.Upgraded.1080p-GRP", ReleaseGroup: "GRP", MediaCoverPath: "42/poster-250.jpg",
			Quality: "Bluray-1080p", PreviousQuality: "HDTV-720p",
			Size: 8_000_000_000, PreviousSize: 2_000_000_000,
			CustomFormatScore: &score, PreviousCustomFormatScore: &previousScore,
			MediaInfo: &arr.MediaInfo{
				Resolution: "1920x1080", VideoCodec: "x265", VideoBitDepth: 10, VideoDynamicRangeType: "HDR10",
				AudioCodec: "TrueHD", AudioChannels: 7.1, AudioBitrate: 4_500_000,
			},
		},
	}

	actLog := &fakeActivityLogger{}
	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		actLog,
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	var found bool
	for _, e := range actLog.entries {
		if e.Action == activity.ActionUpgradeDetected {
			found = true
			if e.Details["itemLabel"] != "Movie.Upgraded" {
				t.Errorf("itemLabel = %v, want Movie.Upgraded", e.Details["itemLabel"])
			}
			if e.Details["itemDetailPath"] != "/movie/movie-upgraded" {
				t.Errorf("itemDetailPath = %v, want /movie/movie-upgraded", e.Details["itemDetailPath"])
			}
			want := map[string]any{
				"releaseTitle":              "Movie.Upgraded.1080p-GRP",
				"releaseGroup":              "GRP",
				"mediaCover":                "42/poster-250.jpg",
				"quality":                   "Bluray-1080p",
				"previousQuality":           "HDTV-720p",
				"size":                      int64(8_000_000_000),
				"previousSize":              int64(2_000_000_000),
				"customFormatScore":         1100,
				"previousCustomFormatScore": 450,
				"resolution":                "1920x1080",
				"videoCodec":                "x265",
				"videoBitDepth":             int64(10),
				"videoDynamicRange":         "HDR10",
				"audioCodec":                "TrueHD",
				"audioChannels":             7.1,
				"audioBitrate":              int64(4_500_000),
			}
			for key, value := range want {
				if e.Details[key] != value {
					t.Errorf("details[%q] = %v (%T), want %v (%T)", key, e.Details[key], e.Details[key], value, value)
				}
			}
			for _, absent := range []string{"videoBitrate", "audioBits", "runTime"} {
				if _, ok := e.Details[absent]; ok {
					t.Errorf("details[%q] present, want omitted for zero value", absent)
				}
			}
			break
		}
	}
	if !found {
		t.Error("expected upgrade_detected activity entry")
	}
}

func TestPollUpgradeHistoryDetectsDownloads(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.historyRecords[instID] = []arr.HistoryRecord{
		{ID: 2, ItemLabel: "Show.S01E01", DetailPath: "/series/the-show", IsUpgrade: false, Quality: "HDTV-720p"},
	}

	actLog := &fakeActivityLogger{}
	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		actLog,
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	var found bool
	for _, e := range actLog.entries {
		if e.Action == activity.ActionDownloadDetected {
			found = true
			if e.Details["itemLabel"] != "Show.S01E01" {
				t.Errorf("itemLabel = %v, want Show.S01E01", e.Details["itemLabel"])
			}
			if e.Details["itemDetailPath"] != "/series/the-show" {
				t.Errorf("itemDetailPath = %v, want /series/the-show", e.Details["itemDetailPath"])
			}
			if _, ok := e.Details["previousQuality"]; ok {
				t.Error("details[previousQuality] present, want omitted for a new download")
			}
			break
		}
	}
	if !found {
		t.Error("expected download_detected activity entry")
	}
}

func TestHistoryPollAdvancesToNewestRecordDate(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr}},
	}

	lastPolled := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	older := lastPolled.Add(time.Minute)
	newest := lastPolled.Add(2 * time.Minute)

	arrSearch := newFakeArrSearcher()
	arrSearch.historyRecords[instID] = []arr.HistoryRecord{
		{ID: 1, ItemID: 10, ItemLabel: "Older", Date: older},
		{ID: 2, ItemID: 11, ItemLabel: "Newest", Date: newest},
	}

	polls := newFakePollTracker()
	polls.lastPolled[instID] = lastPolled

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{}, arrSearch, polls,
	)

	_ = runScheduler(t, sched)

	if got := polls.lastPolled[instID]; !got.Equal(newest) {
		t.Errorf("lastPolled = %v, want the newest record date %v", got, newest)
	}
}

func TestHistoryPollWithoutRecordsKeepsPollTime(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr}},
	}

	lastPolled := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	polls := newFakePollTracker()
	polls.lastPolled[instID] = lastPolled

	sched := newTestScheduler(t,
		instances, newFakeSettingsResolver(), newFakeCooldownTracker(), &fakeActivityLogger{},
		newFakeArrSearcher(), polls,
	)

	_ = runScheduler(t, sched)

	if got := polls.lastPolled[instID]; !got.Equal(lastPolled) {
		t.Errorf("lastPolled = %v, want it unchanged at %v", got, lastPolled)
	}
}

func TestMissingItemsIncludedWhenEnabled(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.missingItems[instID] = []arr.UpgradeItem{
		{ID: 10, Label: "Missing Movie"},
	}

	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) == 0 {
		t.Fatal("expected at least one search call")
	}
	call := arrSearch.searchCalls[0]
	if len(call.itemIDs) != 1 || call.itemIDs[0] != 10 {
		t.Errorf("searched items = %v, want [10]", call.itemIDs)
	}
}

func TestMissingItemsExcludedWhenDisabled(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Radarr", AppType: instance.AppTypeRadarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	noMissing := settings.Defaults()
	noMissing.SearchMissing = false
	settingsRes.perInst[instID] = noMissing

	arrSearch := newFakeArrSearcher()
	arrSearch.missingItems[instID] = []arr.UpgradeItem{
		{ID: 10, Label: "Missing Movie"},
	}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) != 0 {
		t.Errorf("searchCalls = %d, want 0 (missing disabled)", len(arrSearch.searchCalls))
	}
}

func TestUpgradeItemsPrioritisedOverMissing(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	settingsRes := newFakeSettingsResolver()
	settingsRes.global.BatchSize = 2

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{
		{ID: 1, Label: "Upgrade 1"},
		{ID: 2, Label: "Upgrade 2"},
	}
	arrSearch.missingItems[instID] = []arr.UpgradeItem{
		{ID: 10, Label: "Missing 1"},
	}

	sched := newTestScheduler(t,
		instances,
		settingsRes,
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if len(arrSearch.searchCalls) == 0 {
		t.Fatal("expected at least one search call")
	}

	call := arrSearch.searchCalls[0]
	if len(call.itemIDs) != 2 {
		t.Fatalf("searched items = %d, want 2 (batch limit)", len(call.itemIDs))
	}
	// Both searched items should be upgrades, not the missing item.
	for _, id := range call.itemIDs {
		if id == 10 {
			t.Error("missing item 10 should not be in batch when upgrades fill it")
		}
	}
}

func TestPollUpgradeHistoryThrottled(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.historyRecords[instID] = []arr.HistoryRecord{
		{ID: 1, ItemLabel: "Movie", IsUpgrade: true, Quality: "HD"},
	}

	polls := newFakePollTracker()
	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		polls,
	)

	synctest.Test(t, func(_ *testing.T) {
		// Simulate that history was just polled.
		sched.mu.Lock()
		sched.lastHistoryPoll = time.Now()
		sched.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), runBudget)
		defer cancel()
		_ = sched.Run(ctx)
	})

	if len(polls.pollCalls) != 0 {
		t.Errorf("pollCalls = %d, want 0 (should be throttled)", len(polls.pollCalls))
	}
}

func TestPruneRetainsCooldownsForLongestPermittedPeriod(t *testing.T) {
	cooldowns := newFakeCooldownTracker()
	sched := newTestScheduler(t,
		&fakeInstanceLister{},
		newFakeSettingsResolver(),
		cooldowns,
		&fakeActivityLogger{},
		newFakeArrSearcher(),
		newFakePollTracker(),
	)

	sched.pruneActivityLog(context.Background(), time.Now())

	if len(cooldowns.deletedOlderThan) != 1 {
		t.Fatalf("DeleteExpired calls = %d, want 1", len(cooldowns.deletedOlderThan))
	}
	if got := cooldowns.deletedOlderThan[0]; got != settings.MaxCooldownPeriod {
		t.Errorf("DeleteExpired olderThan = %v, want %v", got, settings.MaxCooldownPeriod)
	}
}

func TestNextSearchAtUsesCycleEndTime(t *testing.T) {
	instID := uuid.New()
	instances := &fakeInstanceLister{
		instances: []instance.Instance{
			{ID: instID, Name: "Sonarr", AppType: instance.AppTypeSonarr},
		},
	}

	arrSearch := newFakeArrSearcher()
	arrSearch.upgradeableItems[instID] = []arr.UpgradeItem{{ID: 1, Label: "Item 1"}}
	arrSearch.upgradeableDelay = 100 * time.Millisecond

	sched := newTestScheduler(t,
		instances,
		newFakeSettingsResolver(),
		newFakeCooldownTracker(),
		&fakeActivityLogger{},
		arrSearch,
		newFakePollTracker(),
	)

	_ = runScheduler(t, sched)

	if arrSearch.upgradeableReturnedAt.IsZero() {
		t.Fatal("expected at least one cycle to run")
	}
	st := sched.Status()
	if len(st.Instances) != 1 {
		t.Fatalf("len(Instances) = %d, want 1", len(st.Instances))
	}

	// The schedule must be computed from the cycle's end, or a slow cycle re-enters the instance back-to-back.
	earliest := arrSearch.upgradeableReturnedAt.Add(settings.Defaults().SearchInterval)
	if st.Instances[0].NextSearchAt.Before(earliest) {
		t.Errorf("NextSearchAt = %v, want not before %v (cycle end + interval)",
			st.Instances[0].NextSearchAt, earliest)
	}
}
