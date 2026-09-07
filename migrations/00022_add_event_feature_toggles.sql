-- +goose Up

ALTER TABLE events
    ADD COLUMN cooking_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN tenting_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE events
    DROP COLUMN IF EXISTS cooking_enabled,
    DROP COLUMN IF EXISTS tenting_enabled;
