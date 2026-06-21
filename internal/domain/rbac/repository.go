package rbac

import "context"

type Repository interface {
	CreateRole(ctx context.Context, role *Role) error
	CreatePermission(ctx context.Context, perm *Permission) error
	AssignRoleToUser(ctx context.Context, userID string, roleID string) error
	RemoveRoleFromUser(ctx context.Context, userID string, roleID string) error
	LinkPermissionToRole(ctx context.Context, roleID string, permID string) error
	GetUserRoles(ctx context.Context, userID string) ([]*Role, error)
	GetUserPermissions(ctx context.Context, userID string) ([]*Permission, error)
	GetRoleByName(ctx context.Context, name string) (*Role, error)
	ListAllRoles(ctx context.Context) ([]*Role, error)
}
