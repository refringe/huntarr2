// schedulerPanel manages the scheduler status widget on the Home page.
// It polls the scheduler API and updates the panel in real time.

// Polling interval in milliseconds for scheduler status refresh.
var SCHEDULER_POLL_INTERVAL_MS = 30000;

// Maximum consecutive fetch failures before showing a connection warning.
var SCHEDULER_MAX_FAILURES = 3;

function schedulerPanel() {
    return {
        status: {
            running: false,
            searchesThisHour: 0,
            hourlyLimit: 0,
            instances: [],
        },
        interval: null,
        _failures: 0,
        connectionLost: false,
        init() {
            this.refresh();
            this.interval = setInterval(
                () => this.refresh(),
                SCHEDULER_POLL_INTERVAL_MS,
            );
        },
        async refresh() {
            if (document.hidden) return;
            try {
                var resp = await fetch('/api/scheduler/status', {
                    signal: AbortSignal.timeout(5000),
                });
                if (resp.ok) {
                    this.status = await resp.json();
                    this._failures = 0;
                    this.connectionLost = false;
                } else {
                    this._failures++;
                }
            } catch (err) {
                console.error('fetching scheduler status:', err);
                this._failures++;
            }
            if (this._failures >= SCHEDULER_MAX_FAILURES) {
                this.connectionLost = true;
            }
        },
        destroy() {
            if (this.interval) clearInterval(this.interval);
        },
    };
}
