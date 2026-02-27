// logViewer manages the activity log table on the Logs page, including
// filtering, pagination, and auto-refresh.

// Polling interval in milliseconds for auto-refresh.
var LOG_POLL_INTERVAL_MS = 5000;

function logViewer() {
    return {
        entries: [],
        total: 0,
        page: 1,
        perPage: 50,
        loading: false,
        fetchError: '',
        autoRefresh: true,
        refreshInterval: null,
        expandedId: null,
        filters: {
            level: '',
            instanceId: '',
            action: '',
            search: '',
            since: '',
            until: '',
        },
        get totalPages() {
            return Math.max(1, Math.ceil(this.total / this.perPage));
        },
        init() {
            this.fetchEntries();
            this.startPolling();
            this.$watch('filters', () => {
                this.page = 1;
                this.fetchEntries();
            });
            this._onVisibility = () => {
                if (document.hidden) {
                    this.stopPolling();
                } else if (this.autoRefresh) {
                    this.startPolling();
                }
            };
            document.addEventListener('visibilitychange', this._onVisibility);
        },
        startPolling() {
            if (this.refreshInterval) return;
            this.refreshInterval = setInterval(
                () => this.fetchEntries(),
                LOG_POLL_INTERVAL_MS,
            );
        },
        stopPolling() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
        },
        async fetchEntries() {
            if (document.hidden) return;
            this.loading = true;
            var params = new URLSearchParams();
            params.set('limit', this.perPage);
            params.set('offset', (this.page - 1) * this.perPage);
            if (this.filters.level) params.set('level', this.filters.level);
            if (this.filters.instanceId) {
                params.set('instanceId', this.filters.instanceId);
            }
            if (this.filters.action) params.set('action', this.filters.action);
            if (this.filters.search) params.set('search', this.filters.search);
            if (this.filters.since) {
                params.set('since', new Date(this.filters.since).toISOString());
            }
            if (this.filters.until) {
                params.set('until', new Date(this.filters.until).toISOString());
            }
            try {
                var resp = await fetch('/api/activity?' + params.toString());
                if (resp.ok) {
                    var data = await resp.json();
                    this.entries = data.entries || [];
                    this.total = data.total || 0;
                    this.fetchError = '';
                } else {
                    this.fetchError = 'Failed to load activity entries.';
                }
            } catch (err) {
                console.error('fetching activity entries:', err);
                this.fetchError = 'Failed to load activity entries.';
            } finally {
                this.loading = false;
            }
        },
        prevPage() {
            if (this.page > 1) {
                this.page--;
                this.fetchEntries();
            }
        },
        nextPage() {
            if (this.page < this.totalPages) {
                this.page++;
                this.fetchEntries();
            }
        },
        toggleAutoRefresh() {
            this.autoRefresh = !this.autoRefresh;
            if (this.autoRefresh) {
                this.startPolling();
            } else {
                this.stopPolling();
            }
        },
        toggleExpand(id) {
            this.expandedId = this.expandedId === id ? null : id;
        },
        hasDetails(entry) {
            return (
                this.searchedItemLinks(entry).length > 0 ||
                this.formatDetails(entry).length > 0
            );
        },
        detailField(entry, key, fallback) {
            if (entry.details && entry.details[key] != null) {
                return String(entry.details[key]);
            }
            return fallback;
        },
        searchedItemLinks(entry) {
            var items = entry.details?.searchedItems;
            if (!items || !items.length) return [];
            var baseURL = entry.details?.instanceBaseURL || '';
            return items.map(function (item) {
                if (typeof item === 'object' && item !== null) {
                    var url =
                        baseURL && item.detailPath
                            ? baseURL + item.detailPath
                            : '';
                    return { label: item.label || '', url: url };
                }
                return { label: String(item), url: '' };
            });
        },
        formatDetails(entry) {
            if (!entry.details) return [];
            var skip = new Set([
                'searchedItems',
                'instanceName',
                'instanceBaseURL',
                'searched',
                'skipped',
                'upgradeableAll',
            ]);
            return Object.entries(entry.details)
                .filter(function (pair) {
                    return !skip.has(pair[0]);
                })
                .map(function (pair) {
                    return [pair[0], String(pair[1])];
                });
        },
        levelClass(level) {
            switch (level) {
                case 'debug':
                    return 'bg-gray-700 text-gray-300';
                case 'info':
                    return 'bg-blue-900 text-blue-300';
                case 'warn':
                    return 'bg-yellow-900 text-yellow-300';
                case 'error':
                    return 'bg-red-900 text-red-300';
                default:
                    return 'bg-gray-700 text-gray-300';
            }
        },
        destroy() {
            this.stopPolling();
            document.removeEventListener(
                'visibilitychange',
                this._onVisibility,
            );
        },
    };
}
