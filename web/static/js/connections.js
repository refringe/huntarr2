// appDefaults maps each application type to its display label and default
// port. Used to generate placeholder URLs and default instance names.
var appDefaults = {
    sonarr: { label: 'Sonarr', port: 8989 },
    radarr: { label: 'Radarr', port: 7878 },
    lidarr: { label: 'Lidarr', port: 8686 },
    whisparr: { label: 'Whisparr', port: 6969 },
};

// defaultPlaceholder returns a placeholder URL for the given app type.
function defaultPlaceholder(appType) {
    var d = appDefaults[appType];
    if (!d) return '';
    return 'http://' + appType + ':' + d.port;
}

// defaultName returns a suggested instance name for the given app type based
// on how many instances of that type already exist on the page. The first
// instance is named after the label (e.g. "Sonarr"), subsequent instances
// append an incrementing suffix (e.g. "Sonarr 2", "Sonarr 3").
var _sectionCache = {};
function defaultName(appType) {
    var d = appDefaults[appType];
    if (!d) return '';
    if (!_sectionCache[appType]) {
        _sectionCache[appType] = document.querySelector(
            'section[data-app-type="' + appType + '"]',
        );
    }
    var section = _sectionCache[appType];
    var count = section
        ? section.querySelectorAll('[data-instance-id]').length
        : 0;
    if (count === 0) return d.label;
    return d.label + ' ' + (count + 1);
}

// connectionManager manages the instance CRUD UI on the Connections page.
function connectionManager() {
    return {
        showModal: false,
        showDeleteModal: false,
        editing: false,
        editId: '',
        deleteId: '',
        deleteName: '',
        testingId: '',
        testResults: {},
        testTimers: {},
        formError: '',
        placeholderUrl: '',
        triggerEl: null,
        form: {
            name: '',
            appType: 'sonarr',
            baseUrl: '',
            apiKey: '',
            timeoutMs: 30000,
        },
        init() {
            var savedScroll = sessionStorage.getItem(
                'huntarr2:connections:scroll',
            );
            if (savedScroll !== null) {
                sessionStorage.removeItem('huntarr2:connections:scroll');
                window.scrollTo(0, parseInt(savedScroll, 10));
            }
            this.$watch('showModal', (open) => {
                if (!open && this.triggerEl) {
                    this.$nextTick(() => this.triggerEl.focus());
                    this.triggerEl = null;
                }
            });
            this.$watch('showDeleteModal', (open) => {
                if (!open && this.triggerEl) {
                    this.$nextTick(() => this.triggerEl.focus());
                    this.triggerEl = null;
                }
            });
        },
        openAdd(appType) {
            this.triggerEl = document.activeElement;
            this.editing = false;
            this.editId = '';
            this.formError = '';
            this.placeholderUrl = defaultPlaceholder(appType);
            this.form = {
                name: defaultName(appType),
                appType: appType,
                baseUrl: '',
                apiKey: '',
                timeoutMs: 30000,
            };
            this.showModal = true;
        },
        async openEdit(id) {
            this.triggerEl = document.activeElement;
            this.editing = true;
            this.editId = id;
            this.formError = '';
            try {
                const resp = await fetch(`/api/instances/${id}`);
                if (this.editId !== id) return;
                if (!resp.ok) {
                    const err = await resp.json().catch(() => ({}));
                    this.formError = err.error || 'Failed to load instance';
                    return;
                }
                const data = await resp.json();
                if (this.editId !== id) return;
                this.placeholderUrl = defaultPlaceholder(data.appType);
                this.form = {
                    name: data.name,
                    appType: data.appType,
                    baseUrl: data.baseUrl,
                    apiKey: '',
                    timeoutMs: data.timeoutMs || 30000,
                };
                this.showModal = true;
            } catch (err) {
                console.error('loading instance:', err);
                this.formError = 'Failed to load instance';
            }
        },
        async saveInstance() {
            this.formError = '';
            const url = this.editing
                ? `/api/instances/${this.editId}`
                : '/api/instances';
            const method = this.editing ? 'PUT' : 'POST';
            try {
                const resp = await fetch(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.form),
                });
                if (!resp.ok) {
                    const err = await resp.json().catch(() => ({}));
                    this.formError = err.error || 'An error occurred';
                    return;
                }
                this.showModal = false;
                this.reloadInstances();
            } catch (err) {
                console.error('saving instance:', err);
                this.formError = 'An error occurred';
            }
        },
        confirmDelete(id, name) {
            this.deleteId = id;
            this.deleteName = name;
            this.showDeleteModal = true;
        },
        confirmDeleteFromEl(el) {
            this.triggerEl = document.activeElement;
            this.confirmDelete(el.dataset.instanceId, el.dataset.instanceName);
        },
        async deleteInstance() {
            try {
                const resp = await fetch(`/api/instances/${this.deleteId}`, {
                    method: 'DELETE',
                });
                this.showDeleteModal = false;
                if (!resp.ok) {
                    const err = await resp.json().catch(() => ({}));
                    this.formError = err.error || 'Failed to delete instance';
                    return;
                }
                this.reloadInstances();
            } catch (err) {
                console.error('deleting instance:', err);
                this.showDeleteModal = false;
                this.formError = 'Failed to delete instance';
            }
        },
        async testInstance(id) {
            this.testingId = id;
            delete this.testResults[id];
            try {
                const resp = await fetch(`/api/instances/${id}/test`, {
                    method: 'POST',
                });
                const data = await resp.json().catch(() => ({}));
                if (resp.ok && data.status === 'ok') {
                    this.testResults[id] = { ok: true, message: 'Connected' };
                } else {
                    this.testResults[id] = {
                        ok: false,
                        message:
                            data.message || data.status || 'Connection failed',
                    };
                }
            } catch (err) {
                console.error('testing instance:', err);
                this.testResults[id] = { ok: false, message: 'Request failed' };
            } finally {
                this.testingId = '';
                if (this.testTimers[id]) clearTimeout(this.testTimers[id]);
                this.testTimers[id] = setTimeout(() => {
                    delete this.testResults[id];
                    delete this.testTimers[id];
                }, 5000);
            }
        },
        reloadInstances() {
            sessionStorage.setItem(
                'huntarr2:connections:scroll',
                String(window.scrollY),
            );
            // Instances are rendered server-side by templ templates, so a full page
            // reload is the correct approach for reflecting mutations. The scroll
            // position is preserved via sessionStorage above.
            window.location.reload();
        },
        destroy() {
            for (var id in this.testTimers) {
                clearTimeout(this.testTimers[id]);
            }
            this.testTimers = {};
        },
    };
}
