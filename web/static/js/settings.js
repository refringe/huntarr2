// settingsManager manages the Settings page form, including global and
// per-instance tabs, save, and reset-to-defaults. When saving instance
// settings, only values that differ from the resolved global settings
// are persisted as per-instance overrides. Values matching global are
// removed so that future global changes propagate automatically.

// Duration in milliseconds before a status message is automatically cleared.
var MESSAGE_DISPLAY_MS = 5000;

// validDuration reports whether v is a valid Go duration string (e.g. "24h",
// "1h30m", "45s"). Returns true for empty strings since clearing the field
// is allowed.
function validDuration(v) {
    if (!v) return true;
    return /^(\d+h)?(\d+m)?(\d+s)?$/.test(v) && v !== '';
}

// validHHMM reports whether v is a valid "HH:MM" time string with exactly
// two digits for both hour and minute. Returns true for empty strings since
// clearing the search window field is allowed.
function validHHMM(v) {
    if (!v) return true;
    return /^([01]\d|2[0-3]):[0-5]\d$/.test(v);
}

function settingsManager() {
    return {
        activeTab: 'global',
        loading: false,
        saving: false,
        message: '',
        messageType: '',
        errors: {},
        globalSettings: null,
        form: {
            batchSize: 10,
            cooldownPeriod: '24h',
            searchWindowStart: '',
            searchWindowEnd: '',
            searchInterval: '6h',
            searchLimit: 100,
            enabled: true,
        },
        init() {
            this.loadSettings();
        },
        selectTab(tab) {
            this.activeTab = tab;
            this.message = '';
            this.errors = {};
            this.loadSettings();
        },
        focusAdjacentTab(event, direction, jump) {
            var tabs = Array.from(
                event.currentTarget.querySelectorAll('[role="tab"]'),
            );
            if (!tabs.length) return;
            var idx = tabs.indexOf(event.target);
            if (idx < 0) return;
            var next;
            if (jump === 'first') {
                next = 0;
            } else if (jump === 'last') {
                next = tabs.length - 1;
            } else {
                next = (idx + direction + tabs.length) % tabs.length;
            }
            tabs[next].focus();
            tabs[next].click();
        },
        validate() {
            this.errors = {};
            if (!validDuration(this.form.cooldownPeriod)) {
                this.errors.cooldownPeriod =
                    'Use a Go duration (e.g. 24h, 1h30m).';
            }
            if (!validDuration(this.form.searchInterval)) {
                this.errors.searchInterval =
                    'Use a Go duration (e.g. 6h, 1h30m).';
            }
            if (!validHHMM(this.form.searchWindowStart)) {
                this.errors.searchWindowStart =
                    'Use HH:MM format (e.g. 01:00).';
            }
            if (!validHHMM(this.form.searchWindowEnd)) {
                this.errors.searchWindowEnd = 'Use HH:MM format (e.g. 06:00).';
            }
            return Object.keys(this.errors).length === 0;
        },
        applyFormData(data) {
            this.form.batchSize = data.batchSize;
            this.form.cooldownPeriod = data.cooldownPeriod;
            this.form.searchWindowStart = data.searchWindowStart || '';
            this.form.searchWindowEnd = data.searchWindowEnd || '';
            this.form.searchInterval = data.searchInterval;
            this.form.searchLimit = data.searchLimit;
            this.form.enabled = data.enabled;
        },
        async loadSettings() {
            this.loading = true;
            try {
                if (this.activeTab === 'global') {
                    var resp = await fetch('/api/settings');
                    if (resp.ok) {
                        var data = await resp.json();
                        this.applyFormData(data);
                        this.globalSettings = data;
                    }
                } else {
                    // Fetch global and instance settings in parallel so the save
                    // logic can compare form values against global.
                    var results = await Promise.all([
                        fetch('/api/settings'),
                        fetch('/api/settings?instanceId=' + this.activeTab),
                    ]);
                    if (results[0].ok) {
                        this.globalSettings = await results[0].json();
                    }
                    if (results[1].ok) {
                        var data = await results[1].json();
                        this.applyFormData(data);
                    }
                }
            } catch (err) {
                console.error('loading settings:', err);
                this.showMessage('Failed to load settings.', 'error');
            } finally {
                this.loading = false;
            }
        },
        async saveSettings() {
            if (!this.validate()) return;
            this.saving = true;
            this.message = '';

            try {
                if (this.activeTab === 'global') {
                    await this.saveGlobalSettings();
                } else {
                    await this.saveInstanceSettings();
                }
            } finally {
                this.saving = false;
            }
        },
        async saveGlobalSettings() {
            var settings = [
                { key: 'batch_size', value: String(this.form.batchSize) },
                { key: 'cooldown_period', value: this.form.cooldownPeriod },
                {
                    key: 'search_window_start',
                    value: this.form.searchWindowStart,
                },
                { key: 'search_window_end', value: this.form.searchWindowEnd },
                { key: 'search_interval', value: this.form.searchInterval },
                { key: 'search_limit', value: String(this.form.searchLimit) },
            ];

            try {
                var resp = await fetch('/api/settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ settings: settings }),
                });
                if (resp.ok) {
                    // Update the cached global settings so instance tabs can
                    // compare against the latest values.
                    this.globalSettings = {
                        batchSize: this.form.batchSize,
                        cooldownPeriod: this.form.cooldownPeriod,
                        searchWindowStart: this.form.searchWindowStart,
                        searchWindowEnd: this.form.searchWindowEnd,
                        searchInterval: this.form.searchInterval,
                        searchLimit: this.form.searchLimit,
                        enabled: this.form.enabled,
                    };
                    this.showMessage('Settings saved.', 'success');
                } else {
                    var err = await resp.json().catch(function () {
                        return {};
                    });
                    this.showMessage(
                        err.error || 'Failed to save settings.',
                        'error',
                    );
                }
            } catch (err) {
                console.error('saving settings:', err);
                this.showMessage('Failed to save settings.', 'error');
            }
        },
        async saveInstanceSettings() {
            var url = '/api/settings?instanceId=' + this.activeTab;
            var gs = this.globalSettings || {};

            // Compare each form value against the resolved global value. Only
            // values that differ become per-instance overrides; values that
            // match global are deleted so global changes propagate.
            var entries = [
                {
                    key: 'batch_size',
                    value: String(this.form.batchSize),
                    global: String(gs.batchSize),
                },
                {
                    key: 'cooldown_period',
                    value: this.form.cooldownPeriod,
                    global: gs.cooldownPeriod || '',
                },
                {
                    key: 'search_window_start',
                    value: this.form.searchWindowStart,
                    global: gs.searchWindowStart || '',
                },
                {
                    key: 'search_window_end',
                    value: this.form.searchWindowEnd,
                    global: gs.searchWindowEnd || '',
                },
                {
                    key: 'search_interval',
                    value: this.form.searchInterval,
                    global: gs.searchInterval || '',
                },
                {
                    key: 'enabled',
                    value: String(this.form.enabled),
                    global: String(gs.enabled),
                },
            ];

            var overrides = [];
            var removals = [];
            for (var i = 0; i < entries.length; i++) {
                if (entries[i].value !== entries[i].global) {
                    overrides.push({
                        key: entries[i].key,
                        value: entries[i].value,
                    });
                } else {
                    removals.push(entries[i].key);
                }
            }

            try {
                if (overrides.length > 0) {
                    var resp = await fetch(url, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ settings: overrides }),
                    });
                    if (!resp.ok) {
                        var err = await resp.json().catch(function () {
                            return {};
                        });
                        this.showMessage(
                            err.error || 'Failed to save settings.',
                            'error',
                        );
                        return;
                    }
                }
                if (removals.length > 0) {
                    var resp2 = await fetch(url, {
                        method: 'DELETE',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ keys: removals }),
                    });
                    if (!resp2.ok) {
                        var err = await resp2.json().catch(function () {
                            return {};
                        });
                        this.showMessage(
                            err.error || 'Failed to save settings.',
                            'error',
                        );
                        return;
                    }
                }
                this.showMessage('Settings saved.', 'success');
            } catch (err) {
                console.error('saving settings:', err);
                this.showMessage('Failed to save settings.', 'error');
            }
        },
        async resetToDefaults() {
            this.saving = true;
            this.message = '';
            var url = '/api/settings?instanceId=' + this.activeTab;
            var keys = [
                'batch_size',
                'cooldown_period',
                'search_window_start',
                'search_window_end',
                'search_interval',
                'enabled',
            ];
            try {
                var resp = await fetch(url, {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ keys: keys }),
                });
                if (resp.ok) {
                    this.showMessage('Reset to defaults.', 'success');
                    this.errors = {};
                    this.loadSettings();
                } else {
                    var err = await resp.json().catch(function () {
                        return {};
                    });
                    this.showMessage(
                        err.error || 'Failed to reset settings.',
                        'error',
                    );
                }
            } catch (err) {
                console.error('resetting settings:', err);
                this.showMessage('Failed to reset settings.', 'error');
            } finally {
                this.saving = false;
            }
        },
        showMessage(msg, type) {
            this.message = msg;
            this.messageType = type;
            clearTimeout(this._messageTimer);
            this._messageTimer = setTimeout(() => {
                this.message = '';
            }, MESSAGE_DISPLAY_MS);
        },
        destroy() {
            clearTimeout(this._messageTimer);
        },
    };
}
