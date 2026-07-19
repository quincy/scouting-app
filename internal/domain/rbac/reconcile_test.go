package rbac

import (
	"context"
	"errors"
	"testing"
)

var errNotFound = errors.New("not found")

type memRBAC struct {
	roles   map[string]*Role
	userIDs []string
}

func (m *memRBAC) CreateRole(_ context.Context, role *Role) error {
	if m.roles == nil {
		m.roles = make(map[string]*Role)
	}
	role.ID = role.Name
	m.roles[role.Name] = role
	return nil
}
func (m *memRBAC) GetRoleByName(_ context.Context, name string) (*Role, error) {
	if m.roles == nil {
		return nil, errNotFound
	}
	r, ok := m.roles[name]
	if !ok {
		return nil, errNotFound
	}
	return r, nil
}
func (m *memRBAC) GetUserRoles(_ context.Context, userID string) ([]*Role, error) {
	if m.roles == nil {
		return nil, nil
	}
	var result []*Role
	for _, r := range m.roles {
		result = append(result, r)
	}
	m.userIDs = append(m.userIDs, userID)
	return result, nil
}
func (m *memRBAC) AssignRoleToUser(_ context.Context, userID, roleID string) error {
	return nil
}
func (m *memRBAC) RemoveRoleFromUser(_ context.Context, userID, roleID string) error {
	return nil
}
func (m *memRBAC) CreatePermission(_ context.Context, _ *Permission) error   { return nil }
func (m *memRBAC) LinkPermissionToRole(_ context.Context, _, _ string) error { return nil }
func (m *memRBAC) GetUserPermissions(_ context.Context, _ string) ([]*Permission, error) {
	return nil, nil
}
func (m *memRBAC) ListAllRoles(_ context.Context) ([]*Role, error) { return nil, nil }
func (m *memRBAC) GetUsersByRoleName(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *memRBAC) ListAllPermissions(_ context.Context) ([]*Permission, error) { return nil, nil }
func (m *memRBAC) GetRolePermissions(_ context.Context, _ string) ([]*Permission, error) {
	return nil, nil
}
func (m *memRBAC) SetRolePermissions(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *memRBAC) GetUsersByPermission(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func TestReconcileRoles_NoPositionsNoRoles(t *testing.T) {
	added, removed, err := ReconcileRoles(context.Background(), &memRBAC{}, "profile-1", "user-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("expected 0 added, got %d: %v", len(added), added)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d: %v", len(removed), removed)
	}
}

func TestReconcileRoles_AddsNewRole(t *testing.T) {
	repo := &memRBAC{}
	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "Treasurer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 || added[0] != "Treasurer" {
		t.Errorf("expected added [Treasurer], got %v", added)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d: %v", len(removed), removed)
	}
}

func TestReconcileRoles_RemovesStaleRole(t *testing.T) {
	repo := &memRBAC{}
	if err := repo.CreateRole(context.Background(), &Role{Name: "Secretary"}); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("expected 0 added, got %d: %v", len(added), added)
	}
	if len(removed) != 1 || removed[0] != "Secretary" {
		t.Errorf("expected removed [Secretary], got %v", removed)
	}
}

func TestReconcileRoles_PreservesProtectedRoles(t *testing.T) {
	repo := &memRBAC{}
	for _, name := range []string{"parent", "admin", "Scouts BSA"} {
		if err := repo.CreateRole(context.Background(), &Role{Name: name}); err != nil {
			t.Fatalf("CreateRole %q: %v", name, err)
		}
	}

	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected protected roles to be preserved, got removed: %v", removed)
	}
	if len(added) != 0 {
		t.Errorf("expected 0 added, got %v", added)
	}
}

func TestReconcileRoles_AddsAndRemovesSimultaneously(t *testing.T) {
	repo := &memRBAC{}
	if err := repo.CreateRole(context.Background(), &Role{Name: "Secretary"}); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "Treasurer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 || added[0] != "Treasurer" {
		t.Errorf("expected added [Treasurer], got %v", added)
	}
	if len(removed) != 1 || removed[0] != "Secretary" {
		t.Errorf("expected removed [Secretary], got %v", removed)
	}
}

func TestReconcileRoles_MultiplePositions(t *testing.T) {
	repo := &memRBAC{}
	if err := repo.CreateRole(context.Background(), &Role{Name: "Secretary"}); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "Treasurer, Historian")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 2 {
		t.Errorf("expected 2 added, got %d: %v", len(added), added)
	}
	if len(removed) != 1 || removed[0] != "Secretary" {
		t.Errorf("expected removed [Secretary], got %v", removed)
	}
}

func TestReconcileRoles_ExistingRoleNotReadded(t *testing.T) {
	repo := &memRBAC{}
	if err := repo.CreateRole(context.Background(), &Role{Name: "Treasurer"}); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	added, removed, err := ReconcileRoles(context.Background(), repo, "profile-1", "user-1", "Treasurer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("expected 0 added (already exists), got %v", added)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %v", removed)
	}
}
