-- +goose Up

-- Seed permissions
INSERT INTO permissions (id, name, created_at, updated_at) VALUES
    (gen_random_uuid(), 'event:create', NOW(), NOW()),
    (gen_random_uuid(), 'event:view', NOW(), NOW()),
    (gen_random_uuid(), 'event:signup', NOW(), NOW()),
    (gen_random_uuid(), 'event:withdraw', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Seed base roles that aren't Scoutbook position roles
INSERT INTO roles (id, name, created_at, updated_at) VALUES
    (gen_random_uuid(), 'admin', NOW(), NOW()),
    (gen_random_uuid(), 'parent', NOW(), NOW()),
    (gen_random_uuid(), 'Scouts BSA', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Link admin role to all permissions
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name IN ('event:create', 'event:view', 'event:signup', 'event:withdraw')
ON CONFLICT DO NOTHING;

-- Link Scoutmaster role to all permissions
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'Scoutmaster'
  AND p.name IN ('event:create', 'event:view', 'event:signup', 'event:withdraw')
ON CONFLICT DO NOTHING;

-- Link Assistant Scoutmaster role to all permissions
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'Assistant Scoutmaster'
  AND p.name IN ('event:create', 'event:view', 'event:signup', 'event:withdraw')
ON CONFLICT DO NOTHING;

-- Link Scouts BSA role to view, signup, withdraw
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'Scouts BSA'
  AND p.name IN ('event:view', 'event:signup', 'event:withdraw')
ON CONFLICT DO NOTHING;

-- Link parent role to view, signup, withdraw
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'parent'
  AND p.name IN ('event:view', 'event:signup', 'event:withdraw')
ON CONFLICT DO NOTHING;

-- +goose Down

-- Remove permission-role links for base roles
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('admin', 'parent', 'Scouts BSA'));

-- Remove Scoutmaster and Asst Scoutmaster links added by this migration
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('Scoutmaster', 'Assistant Scoutmaster'));

-- Remove base roles
DELETE FROM roles WHERE name IN ('admin', 'parent', 'Scouts BSA');

-- Remove permissions
DELETE FROM permissions WHERE name IN ('event:create', 'event:view', 'event:signup', 'event:withdraw');
