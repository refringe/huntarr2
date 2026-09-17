-- +goose NO TRANSACTION

-- Rebuilds the instances table with the "whisparr" app type split into "whisparr-v2" and "whisparr-v3".

-- +goose Up

PRAGMA foreign_keys = OFF;

CREATE TABLE instances_new (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    app_type    TEXT NOT NULL CHECK (app_type IN (
                    'sonarr', 'radarr', 'lidarr',
                    'whisparr-v2', 'whisparr-v3'
                )),
    base_url    TEXT NOT NULL,
    api_key_enc TEXT NOT NULL,
    timeout_ms  INTEGER NOT NULL DEFAULT 30000,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO instances_new (
    id, name, app_type, base_url, api_key_enc, timeout_ms, created_at, updated_at
)
SELECT id, name,
       CASE app_type WHEN 'whisparr' THEN 'whisparr-v2' ELSE app_type END,
       base_url, api_key_enc, timeout_ms, created_at, updated_at
FROM instances;

DROP TABLE instances;

ALTER TABLE instances_new RENAME TO instances;

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

PRAGMA foreign_keys = ON;

-- +goose Down

PRAGMA foreign_keys = OFF;

CREATE TABLE instances_old (
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

INSERT INTO instances_old (
    id, name, app_type, base_url, api_key_enc, timeout_ms, created_at, updated_at
)
SELECT id, name,
       CASE app_type
           WHEN 'whisparr-v2' THEN 'whisparr'
           WHEN 'whisparr-v3' THEN 'whisparr'
           ELSE app_type
       END,
       base_url, api_key_enc, timeout_ms, created_at, updated_at
FROM instances;

DROP TABLE instances;

ALTER TABLE instances_old RENAME TO instances;

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

PRAGMA foreign_keys = ON;
