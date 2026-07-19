-- +goose Up
ALTER TABLE profiles DROP CONSTRAINT chk_profile_status;
ALTER TABLE profiles ADD CONSTRAINT chk_profile_status CHECK (status IN ('active', 'inactive', 'disabled'));

-- +goose Down
ALTER TABLE profiles DROP CONSTRAINT chk_profile_status;
ALTER TABLE profiles ADD CONSTRAINT chk_profile_status CHECK (status IN ('active', 'inactive'));
