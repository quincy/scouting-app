-- +goose Up

CREATE TABLE event_tents (
    id         UUID NOT NULL PRIMARY KEY,
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_event_tents_event_id
    ON event_tents (event_id);

CREATE TABLE event_tent_members (
    event_id   UUID NOT NULL,
    profile_id UUID NOT NULL,
    tent_id    UUID NOT NULL REFERENCES event_tents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, profile_id),
    FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id)
);

CREATE INDEX idx_event_tent_members_tent_id
    ON event_tent_members (tent_id);

-- +goose Down

DROP TABLE IF EXISTS event_tent_members;
DROP TABLE IF EXISTS event_tents;
