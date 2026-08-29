-- +goose Up

CREATE TABLE event_cooking_patrols (
    id         UUID NOT NULL PRIMARY KEY,
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    is_adult   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Patrol names are unique per event, which also serves event_id lookups
CREATE UNIQUE INDEX idx_event_cooking_patrols_event_name
    ON event_cooking_patrols (event_id, name);

-- At most one "Adults" cooking patrol per event
CREATE UNIQUE INDEX idx_event_cooking_patrols_single_adult
    ON event_cooking_patrols (event_id)
    WHERE is_adult;

CREATE TABLE event_cooking_patrol_members (
    event_id   UUID NOT NULL,
    profile_id UUID NOT NULL,
    patrol_id  UUID NOT NULL,
    is_cook    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, profile_id),
    FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id) ON DELETE CASCADE,
    FOREIGN KEY (patrol_id) REFERENCES event_cooking_patrols(id) ON DELETE CASCADE
);

CREATE INDEX idx_event_cooking_patrols_members_patrol_id
    ON event_cooking_patrol_members (patrol_id);

-- At most one Cook per cooking patrol
CREATE UNIQUE INDEX idx_event_cooking_patrol_single_cook
    ON event_cooking_patrol_members (patrol_id)
    WHERE is_cook;

-- +goose Down

DROP TABLE IF EXISTS event_cooking_patrol_members;
DROP TABLE IF EXISTS event_cooking_patrols;