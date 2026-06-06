-- +goose Up

DROP TABLE IF EXISTS event_attendee_responsibilities;
DROP TABLE IF EXISTS event_attendees;

CREATE TABLE event_attendees (
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'signed_up',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, profile_id),
    CONSTRAINT chk_attendee_status CHECK (status IN ('signed_up', 'canceled'))
);

CREATE TABLE event_attendee_responsibilities (
    event_id       UUID NOT NULL,
    profile_id     UUID NOT NULL,
    responsibility TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, profile_id, responsibility),
    FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id) ON DELETE CASCADE,
    CONSTRAINT chk_responsibility CHECK (responsibility IN ('driver', 'cook'))
);

-- +goose Down

DROP TABLE IF EXISTS event_attendee_responsibilities;
DROP TABLE IF EXISTS event_attendees;

CREATE TABLE event_attendees (
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'signed_up',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, user_id),
    CONSTRAINT chk_attendee_status CHECK (status IN ('signed_up', 'canceled'))
);

CREATE TABLE event_attendee_responsibilities (
    event_id       UUID NOT NULL,
    user_id        UUID NOT NULL,
    responsibility TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, user_id, responsibility),
    FOREIGN KEY (event_id, user_id) REFERENCES event_attendees(event_id, user_id) ON DELETE CASCADE,
    CONSTRAINT chk_responsibility CHECK (responsibility IN ('driver', 'cook'))
);
