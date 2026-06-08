-- +goose Up

ALTER TABLE profiles ADD COLUMN nickname TEXT NOT NULL DEFAULT '';
ALTER TABLE profiles ADD COLUMN gender   TEXT NOT NULL DEFAULT '';
ALTER TABLE profiles ADD COLUMN positions TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE profiles DROP COLUMN nickname;
ALTER TABLE profiles DROP COLUMN gender;
ALTER TABLE profiles DROP COLUMN positions;
