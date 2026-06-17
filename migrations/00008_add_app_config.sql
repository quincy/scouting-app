-- +goose Up
CREATE TABLE app_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS app_config;
