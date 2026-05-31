package domain

import "context"

// UserRepository handles user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}

// RBACRepository handles role-based authorization rules.
type RBACRepository interface {
	CreateRole(ctx context.Context, role *Role) error
	CreatePermission(ctx context.Context, perm *Permission) error
	AssignRoleToUser(ctx context.Context, userID string, roleID string) error
	LinkPermissionToRole(ctx context.Context, roleID string, permID string) error
	GetUserRoles(ctx context.Context, userID string) ([]*Role, error)
	GetUserPermissions(ctx context.Context, userID string) ([]*Permission, error)
}

// EventRepository handles events, chronological views, and sign-ups.
type EventRepository interface {
	Create(ctx context.Context, event *Event) error
	GetByID(ctx context.Context, id string) (*Event, error)
	ListUpcoming(ctx context.Context, limit int, offset int) ([]*Event, error)
	ListPast(ctx context.Context, limit int, offset int) ([]*Event, error)
	SignUp(ctx context.Context, eventID string, userID string) error
	Withdraw(ctx context.Context, eventID string, userID string) error
	GetAttendees(ctx context.Context, eventID string) ([]*User, error)
}
