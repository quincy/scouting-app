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
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = $1`, role.Name,
	).Scan(&role.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return r.db.QueryRowContext(ctx,
			`INSERT INTO roles (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())
			 RETURNING id`,
			newUUID(), role.Name,
		).Scan(&role.ID)
	}
	return err
}

func (r *RBACRepository) CreatePermission(ctx context.Context, perm *rbac.Permission) error {
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM permissions WHERE name = $1`, perm.Name,
	).Scan(&perm.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return r.db.QueryRowContext(ctx,
			`INSERT INTO permissions (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())
			 RETURNING id`,
			newUUID(), perm.Name,
		).Scan(&perm.ID)
	}
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

func (r *RBACRepository) RemoveRoleFromUser(ctx context.Context, userID string, roleID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`,
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

func (r *RBACRepository) ListAllRoles(ctx context.Context) ([]*rbac.Role, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM roles ORDER BY name`)
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

func (r *RBACRepository) GetUsersByRoleName(ctx context.Context, name string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ur.user_id
		 FROM user_roles ur
		 JOIN roles r ON r.id = ur.role_id
		 WHERE r.name = $1`, name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

var _ rbac.Repository = (*RBACRepository)(nil)
