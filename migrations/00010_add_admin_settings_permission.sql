-- +goose Up

-- Add admin:settings permission
INSERT INTO permissions (id, name, created_at, updated_at) VALUES
    (gen_random_uuid(), 'admin:settings', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Link admin:settings to admin role
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, NOW()
FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name = 'admin:settings'
ON CONFLICT DO NOTHING;

-- +goose Down

-- Remove admin:settings from admin role
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE name = 'admin:settings');

-- Remove the permission itself
DELETE FROM permissions WHERE name = 'admin:settings';
