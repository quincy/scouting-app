-- +goose Up

-- Add admin:roster, admin:connections, admin:sync permissions
INSERT INTO permissions (id, name, created_at, updated_at) VALUES
    (gen_random_uuid(), 'admin:roster', NOW(), NOW()),
    (gen_random_uuid(), 'admin:connections', NOW(), NOW()),
    (gen_random_uuid(), 'admin:sync', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Link all three to admin role
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name IN ('admin:roster', 'admin:connections', 'admin:sync')
ON CONFLICT DO NOTHING;

-- +goose Down

-- Remove from admin role
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('admin:roster', 'admin:connections', 'admin:sync')
);

-- Remove the permissions
DELETE FROM permissions WHERE name IN ('admin:roster', 'admin:connections', 'admin:sync');
