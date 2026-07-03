-- +goose Up

-- Seed SMTP config keys with empty defaults (values set via settings page)
INSERT INTO app_config (key, value) VALUES
    ('SMTP_HOST', ''),
    ('SMTP_PORT', ''),
    ('SMTP_USER', ''),
    ('SMTP_PASS', ''),
    ('SMTP_FROM', '')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM app_config WHERE key IN ('SMTP_HOST', 'SMTP_PORT', 'SMTP_USER', 'SMTP_PASS', 'SMTP_FROM');
