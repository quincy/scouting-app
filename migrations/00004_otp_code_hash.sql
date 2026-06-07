-- +goose Up

ALTER TABLE otp_codes ADD COLUMN code_hash BYTEA;
UPDATE otp_codes SET code_hash = decode(sha256(code), 'hex');
ALTER TABLE otp_codes ALTER COLUMN code_hash SET NOT NULL;

ALTER TABLE otp_codes ADD COLUMN attempts INT NOT NULL DEFAULT 0;

ALTER TABLE otp_codes DROP COLUMN code;

-- +goose Down

ALTER TABLE otp_codes ADD COLUMN code TEXT;
ALTER TABLE otp_codes DROP COLUMN code_hash;
ALTER TABLE otp_codes DROP COLUMN attempts;
