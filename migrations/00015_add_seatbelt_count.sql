-- +goose Up
ALTER TABLE event_attendee_responsibilities ADD COLUMN seatbelt_count INT;

-- +goose Down
ALTER TABLE event_attendee_responsibilities DROP COLUMN seatbelt_count;
