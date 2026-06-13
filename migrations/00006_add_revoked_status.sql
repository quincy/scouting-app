-- +goose Up
ALTER TABLE parent_youth_links DROP CONSTRAINT chk_link_status;
ALTER TABLE parent_youth_links ADD CONSTRAINT chk_link_status CHECK (status IN ('pending', 'approved', 'rejected', 'revoked'));

-- +goose Down
ALTER TABLE parent_youth_links DROP CONSTRAINT chk_link_status;
ALTER TABLE parent_youth_links ADD CONSTRAINT chk_link_status CHECK (status IN ('pending', 'approved', 'rejected'));
