package postgres

import (
	"context"
	"testing"

	"scout-app/internal/domain/rbac"
)

func TestPostgresRBACRepository_RolesAndPermissions(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewRBACRepository(testDB)
	ctx := context.Background()

	adminRole := &rbac.Role{Name: "Admin"}
	leaderRole := &rbac.Role{Name: "Leader"}
	if err := repo.CreateRole(ctx, adminRole); err != nil {
		t.Fatalf("CreateRole admin: %v", err)
	}
	if err := repo.CreateRole(ctx, leaderRole); err != nil {
		t.Fatalf("CreateRole leader: %v", err)
	}

	if adminRole.ID == "" {
		t.Error("expected generated UUID for role")
	}

	perm1 := &rbac.Permission{Name: "event:create"}
	perm2 := &rbac.Permission{Name: "event:signup"}
	if err := repo.CreatePermission(ctx, perm1); err != nil {
		t.Fatalf("CreatePermission: %v", err)
	}
	if err := repo.CreatePermission(ctx, perm2); err != nil {
		t.Fatalf("CreatePermission: %v", err)
	}

	if err := repo.LinkPermissionToRole(ctx, adminRole.ID, perm1.ID); err != nil {
		t.Fatalf("LinkPermissionToRole: %v", err)
	}
	if err := repo.LinkPermissionToRole(ctx, adminRole.ID, perm2.ID); err != nil {
		t.Fatalf("LinkPermissionToRole: %v", err)
	}
	if err := repo.LinkPermissionToRole(ctx, leaderRole.ID, perm2.ID); err != nil {
		t.Fatalf("LinkPermissionToRole: %v", err)
	}

	userID := "test-user-uuid"
	if err := repo.AssignRoleToUser(ctx, userID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser admin: %v", err)
	}
	if err := repo.AssignRoleToUser(ctx, userID, leaderRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser leader: %v", err)
	}

	roles, err := repo.GetUserRoles(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}

	perms, err := repo.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions: %v", err)
	}
	if len(perms) != 2 {
		t.Errorf("expected 2 unique permissions, got %d", len(perms))
	}
}

func TestPostgresRBACRepository_GetRoleByName(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewRBACRepository(testDB)
	ctx := context.Background()

	role := &rbac.Role{Name: "TestRole"}
	if err := repo.CreateRole(ctx, role); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	fetched, err := repo.GetRoleByName(ctx, "TestRole")
	if err != nil {
		t.Fatalf("GetRoleByName: %v", err)
	}
	if fetched.ID != role.ID {
		t.Errorf("expected ID %s, got %s", role.ID, fetched.ID)
	}

	_, err = repo.GetRoleByName(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent role")
	}
}

func TestPostgresRBACRepository_AssignRoleIdempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewRBACRepository(testDB)
	ctx := context.Background()

	role := &rbac.Role{Name: "testrole"}
	repo.CreateRole(ctx, role)

	if err := repo.AssignRoleToUser(ctx, "user1", role.ID); err != nil {
		t.Fatalf("first assign: %v", err)
	}
	if err := repo.AssignRoleToUser(ctx, "user1", role.ID); err != nil {
		t.Fatalf("second assign (idempotent) should not error: %v", err)
	}

	roles, _ := repo.GetUserRoles(ctx, "user1")
	if len(roles) != 1 {
		t.Errorf("expected 1 role after duplicate assign, got %d", len(roles))
	}
}
