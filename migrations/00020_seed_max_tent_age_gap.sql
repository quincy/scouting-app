-- +goose Up

-- Seed the unit-wide max tent age gap (in whole years) with the default of 2.
-- Exposed on the admin settings page; no per-event override.
INSERT INTO app_config (key, value) VALUES
    ('MAX_TENT_AGE_GAP', '2')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM app_config WHERE key = 'MAX_TENT_AGE_GAP';