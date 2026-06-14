package postgres

import (
	"context"
	"database/sql"
	"errors"

	"scout-app/internal/domain/rbac"
)

type RBACRepository struct {
	db *sql.DB
}

func NewRBACRepository(db *sql.DB) *RBACRepository {
	return &RBACRepository{db: db}
}

func (r *RBACRepository) CreateRole(ctx context.Context, role *rbac.Role) error {
	if role.ID == "" {
		role.ID = newUUID()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO roles (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())
		 ON CONFLICT (name) DO NOTHING`,
		role.ID, role.Name,
	)
	return err
}

func (r *RBACRepository) CreatePermission(ctx context.Context, perm *rbac.Permission) error {
	if perm.ID == "" {
		perm.ID = newUUID()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO permissions (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())
		 ON CONFLICT (name) DO NOTHING`,
		perm.ID, perm.Name,
	)
	return err
}

func (r *RBACRepository) AssignRoleToUser(ctx context.Context, userID string, roleID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id, created_at) VALUES ($1, $2, NOW())
		 ON CONFLICT DO NOTHING`,
		userID, roleID,
	)
	return err
}

func (r *RBACRepository) LinkPermissionToRole(ctx context.Context, roleID string, permID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO role_permissions (role_id, permission_id, created_at) VALUES ($1, $2, NOW())
		 ON CONFLICT DO NOTHING`,
		roleID, permID,
	)
	return err
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID string) ([]*rbac.Role, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT r.id, r.name
		 FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*rbac.Role
	for rows.Next() {
		rl := &rbac.Role{}
		if err := rows.Scan(&rl.ID, &rl.Name); err != nil {
			return nil, err
		}
		roles = append(roles, rl)
	}
	return roles, rows.Err()
}

func (r *RBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]*rbac.Permission, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT p.id, p.name
		 FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*rbac.Permission
	for rows.Next() {
		perm := &rbac.Permission{}
		if err := rows.Scan(&perm.ID, &perm.Name); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, rows.Err()
}

func (r *RBACRepository) GetRoleByName(ctx context.Context, name string) (*rbac.Role, error) {
	rl := &rbac.Role{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name FROM roles WHERE name = $1`, name,
	).Scan(&rl.ID, &rl.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("role not found")
	}
	return rl, err
}

var _ rbac.Repository = (*RBACRepository)(nil)
