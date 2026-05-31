package mock

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"scout-app/internal/domain"
	"sync"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// UserRepository is an in-memory implementation of domain.UserRepository.
type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*domain.User),
	}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if u.Email == user.Email {
			return fmt.Errorf("email %q already exists", user.Email)
		}
	}

	if user.ID == "" {
		user.ID = newUUID()
	}
	r.users[user.ID] = user
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

// RBACRepository is an in-memory implementation of domain.RBACRepository.
type RBACRepository struct {
	mu              sync.RWMutex
	roles           map[string]*domain.Role
	permissions     map[string]*domain.Permission
	userRoles       map[string][]string // userID -> []roleID
	rolePermissions map[string][]string // roleID -> []permID
}

func NewRBACRepository() *RBACRepository {
	return &RBACRepository{
		roles:           make(map[string]*domain.Role),
		permissions:     make(map[string]*domain.Permission),
		userRoles:       make(map[string][]string),
		rolePermissions: make(map[string][]string),
	}
}

func (r *RBACRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rl := range r.roles {
		if rl.Name == role.Name {
			return fmt.Errorf("role %q already exists", role.Name)
		}
	}

	if role.ID == "" {
		role.ID = newUUID()
	}
	r.roles[role.ID] = role
	return nil
}

func (r *RBACRepository) CreatePermission(ctx context.Context, perm *domain.Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.permissions {
		if p.Name == perm.Name {
			return fmt.Errorf("permission %q already exists", perm.Name)
		}
	}

	if perm.ID == "" {
		perm.ID = newUUID()
	}
	r.permissions[perm.ID] = perm
	return nil
}

func (r *RBACRepository) AssignRoleToUser(ctx context.Context, userID string, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if role exists
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}

	// Prevent duplicates
	for _, rid := range r.userRoles[userID] {
		if rid == roleID {
			return nil
		}
	}

	r.userRoles[userID] = append(r.userRoles[userID], roleID)
	return nil
}

func (r *RBACRepository) LinkPermissionToRole(ctx context.Context, roleID string, permID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if role and permission exist
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}
	if _, ok := r.permissions[permID]; !ok {
		return errors.New("permission not found")
	}

	// Prevent duplicates
	for _, pid := range r.rolePermissions[roleID] {
		if pid == permID {
			return nil
		}
	}

	r.rolePermissions[roleID] = append(r.rolePermissions[roleID], permID)
	return nil
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID string) ([]*domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rids := r.userRoles[userID]
	var roles []*domain.Role
	for _, rid := range rids {
		if role, ok := r.roles[rid]; ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *RBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]*domain.Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rids := r.userRoles[userID]
	permSet := make(map[string]bool)
	var permissions []*domain.Permission

	for _, rid := range rids {
		pids := r.rolePermissions[rid]
		for _, pid := range pids {
			if !permSet[pid] {
				permSet[pid] = true
				if perm, ok := r.permissions[pid]; ok {
					permissions = append(permissions, perm)
				}
			}
		}
	}
	return permissions, nil
}
