-- +goose Up
-- +goose StatementBegin

CREATE TABLE profiles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bsa_id      TEXT NOT NULL DEFAULT '',
    first_name  TEXT NOT NULL,
    last_name   TEXT NOT NULL,
    email       TEXT NOT NULL DEFAULT '',
    phone       TEXT NOT NULL DEFAULT '',
    birthdate   DATE NOT NULL DEFAULT '1970-01-01',
    member_type TEXT NOT NULL DEFAULT 'adult',
    status      TEXT NOT NULL DEFAULT 'active',
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_member_type CHECK (member_type IN ('adult', 'youth')),
    CONSTRAINT chk_profile_status CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX idx_profiles_bsa_id ON profiles (bsa_id) WHERE bsa_id != '';
CREATE INDEX idx_profiles_email ON profiles (email);
CREATE INDEX idx_profiles_user_id ON profiles (user_id);

CREATE TABLE otp_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    code       TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_otp_codes_email ON otp_codes (email);

CREATE TABLE parent_youth_links (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    youth_profile_id  UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    status            TEXT NOT NULL DEFAULT 'pending',
    requested_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at       TIMESTAMPTZ,
    approved_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_link_status CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX idx_ply_parent ON parent_youth_links (parent_profile_id);
CREATE INDEX idx_ply_youth ON parent_youth_links (youth_profile_id);

CREATE TABLE scoutbook_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token       TEXT NOT NULL,
    person_guid TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO profiles (first_name, last_name, email, member_type, status, user_id)
SELECT 'Imported', email, email, 'adult', 'active', id
FROM users
WHERE NOT EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id = users.id);

DROP INDEX IF EXISTS idx_users_email;

ALTER TABLE users DROP COLUMN email;

ALTER TABLE event_attendees ADD COLUMN profile_id UUID;

UPDATE event_attendees ea
SET profile_id = p.id
FROM profiles p
WHERE p.user_id = ea.user_id;

ALTER TABLE event_attendees ALTER COLUMN profile_id SET NOT NULL;

ALTER TABLE event_attendees ADD CONSTRAINT fk_event_attendees_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE;

ALTER TABLE event_attendee_responsibilities DROP CONSTRAINT IF EXISTS event_attendee_responsibilities_event_id_fkey;

ALTER TABLE event_attendees DROP CONSTRAINT event_attendees_pkey;
ALTER TABLE event_attendees DROP COLUMN user_id;
ALTER TABLE event_attendees ADD PRIMARY KEY (event_id, profile_id);

ALTER TABLE event_attendee_responsibilities ADD COLUMN profile_id UUID;

UPDATE event_attendee_responsibilities ear
SET profile_id = p.id
FROM profiles p
WHERE p.user_id = ear.user_id;

ALTER TABLE event_attendee_responsibilities ALTER COLUMN profile_id SET NOT NULL;

ALTER TABLE event_attendee_responsibilities DROP CONSTRAINT event_attendee_responsibilities_pkey;
ALTER TABLE event_attendee_responsibilities DROP COLUMN user_id;
ALTER TABLE event_attendee_responsibilities ADD PRIMARY KEY (event_id, profile_id, responsibility);
ALTER TABLE event_attendee_responsibilities ADD FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE event_attendee_responsibilities DROP CONSTRAINT IF EXISTS event_attendee_responsibilities_event_id_fkey;
ALTER TABLE event_attendee_responsibilities DROP CONSTRAINT event_attendee_responsibilities_pkey;

ALTER TABLE event_attendee_responsibilities ADD COLUMN user_id UUID;
UPDATE event_attendee_responsibilities ear
SET user_id = p.user_id
FROM profiles p
WHERE p.id = ear.profile_id;
ALTER TABLE event_attendee_responsibilities ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE event_attendee_responsibilities DROP COLUMN profile_id;
ALTER TABLE event_attendee_responsibilities ADD PRIMARY KEY (event_id, user_id, responsibility);
ALTER TABLE event_attendee_responsibilities ADD FOREIGN KEY (event_id, user_id) REFERENCES event_attendees(event_id, user_id) ON DELETE CASCADE;

ALTER TABLE event_attendees DROP CONSTRAINT fk_event_attendees_profile;
ALTER TABLE event_attendees DROP CONSTRAINT event_attendees_pkey;

ALTER TABLE event_attendees ADD COLUMN user_id UUID;
UPDATE event_attendees ea
SET user_id = p.user_id
FROM profiles p
WHERE p.id = ea.profile_id;
ALTER TABLE event_attendees ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE event_attendees DROP COLUMN profile_id;
ALTER TABLE event_attendees ADD PRIMARY KEY (event_id, user_id);

ALTER TABLE users ADD COLUMN email TEXT;
UPDATE users u
SET email = p.email
FROM profiles p
WHERE p.user_id = u.id;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
CREATE UNIQUE INDEX idx_users_email ON users (email);

DROP TABLE IF EXISTS scoutbook_sessions;
DROP TABLE IF EXISTS parent_youth_links;
DROP TABLE IF EXISTS otp_codes;
DROP TABLE IF EXISTS profiles;

-- +goose StatementEnd
