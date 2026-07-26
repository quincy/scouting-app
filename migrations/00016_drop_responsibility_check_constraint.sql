-- +goose Up
ALTER TABLE event_attendee_responsibilities DROP CONSTRAINT IF EXISTS chk_responsibility;

-- +goose Down
ALTER TABLE event_attendee_responsibilities ADD CONSTRAINT chk_responsibility CHECK (responsibility IN ('driver', 'cook'));
