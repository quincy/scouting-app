-- +goose Up

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

-- +goose Down

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
