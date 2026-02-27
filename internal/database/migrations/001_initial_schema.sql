-- +goose Up

CREATE TABLE instances (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    app_type    TEXT NOT NULL CHECK (app_type IN (
                    'sonarr', 'radarr',
                    'lidarr', 'whisparr'
                )),
    base_url    TEXT NOT NULL,
    api_key_enc TEXT NOT NULL,
    timeout_ms  INTEGER NOT NULL DEFAULT 30000,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_instances_app_type ON instances (app_type);

-- +goose StatementBegin
CREATE TRIGGER trg_instances_updated_at
    AFTER UPDATE ON instances
    FOR EACH ROW
BEGIN
    UPDATE instances SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE settings (
    id           TEXT PRIMARY KEY,
    instance_id  TEXT REFERENCES instances (id) ON DELETE CASCADE,
    setting_key  TEXT NOT NULL,
    value        TEXT NOT NULL,
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX idx_settings_global_key
    ON settings (setting_key) WHERE instance_id IS NULL;
CREATE UNIQUE INDEX idx_settings_instance_key
    ON settings (instance_id, setting_key) WHERE instance_id IS NOT NULL;
CREATE INDEX idx_settings_instance_id ON settings (instance_id);

-- +goose StatementBegin
CREATE TRIGGER trg_settings_updated_at
    AFTER UPDATE ON settings
    FOR EACH ROW
BEGIN
    UPDATE settings SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE search_cooldowns (
    instance_id  TEXT NOT NULL REFERENCES instances (id) ON DELETE CASCADE,
    item_id      INTEGER NOT NULL,
    searched_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (instance_id, item_id)
);

CREATE INDEX idx_search_cooldowns_instance_searched
    ON search_cooldowns (instance_id, searched_at);

CREATE TABLE history_poll_state (
    instance_id  TEXT PRIMARY KEY REFERENCES instances (id) ON DELETE CASCADE,
    last_polled  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose StatementBegin
CREATE TRIGGER trg_history_poll_state_updated_at
    AFTER UPDATE ON history_poll_state
    FOR EACH ROW
BEGIN
    UPDATE history_poll_state SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE instance_id = NEW.instance_id;
END;
-- +goose StatementEnd

CREATE TABLE activity_log (
    id           TEXT PRIMARY KEY,
    instance_id  TEXT REFERENCES instances (id) ON DELETE SET NULL,
    level        TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    action       TEXT NOT NULL CHECK (action IN (
                     'search_cycle', 'search_skip',
                     'rate_limit', 'health_check',
                     'upgrade_detected', 'download_detected'
                 )),
    message      TEXT NOT NULL,
    details      TEXT,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_activity_log_created_at ON activity_log (created_at);
CREATE INDEX idx_activity_log_instance_id ON activity_log (instance_id);
CREATE INDEX idx_activity_log_level ON activity_log (level);
CREATE INDEX idx_activity_log_action_created ON activity_log (action, created_at);

-- +goose Down

DROP TABLE IF EXISTS activity_log;
DROP TABLE IF EXISTS history_poll_state;
DROP TABLE IF EXISTS search_cooldowns;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS instances;
