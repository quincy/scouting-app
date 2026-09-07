-- +goose Up

ALTER TABLE events
    ADD COLUMN drivers_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE events
    DROP COLUMN IF EXISTS drivers_enabled;