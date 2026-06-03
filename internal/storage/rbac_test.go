package storage

import (
	"context"
	"testing"

	"scout-app/internal/domain/rbac"
	"scout-app/internal/storage/mock"
)

func TestRBACRepository_RolesAndPermissions(t *testing.T) {
	repo := mock.NewRBACRepository()
	ctx := context.Background()

	// 1. Create Roles
	adminRole := &rbac.Role{Name: "Admin"}
	leaderRole := &rbac.Role{Name: "Leader"}
	err := repo.CreateRole(ctx, adminRole)
	if err != nil {
		t.Fatalf("failed to create admin role: %v", err)
	}
	_ = repo.CreateRole(ctx, leaderRole)

	if adminRole.ID == "" {
		t.Error("expected admin role to have generated UUID ID")
	}

	// Try creating duplicate role
	err = repo.CreateRole(ctx, &rbac.Role{Name: "Admin"})
	if err == nil {
		t.Error("expected error when creating duplicate role, got nil")
	}

	// 2. Create Permissions
	createEventPerm := &rbac.Permission{Name: "Create Event"}
	signUpPerm := &rbac.Permission{Name: "Sign up for Event"}
	err = repo.CreatePermission(ctx, createEventPerm)
	if err != nil {
		t.Fatalf("failed to create permission: %v", err)
	}
	_ = repo.CreatePermission(ctx, signUpPerm)

	if createEventPerm.ID == "" {
		t.Error("expected permission to have generated UUID ID")
	}

	// 3. Link permissions to roles
	// Admin gets "Create Event" and "Sign up for Event"
	err = repo.LinkPermissionToRole(ctx, adminRole.ID, createEventPerm.ID)
	if err != nil {
		t.Fatalf("failed to link permission to admin: %v", err)
	}
	_ = repo.LinkPermissionToRole(ctx, adminRole.ID, signUpPerm.ID)

	// Leader gets "Sign up for Event" only
	_ = repo.LinkPermissionToRole(ctx, leaderRole.ID, signUpPerm.ID)

	// 4. Assign roles to user
	userID := "user-uuid-123"
	err = repo.AssignRoleToUser(ctx, userID, adminRole.ID)
	if err != nil {
		t.Fatalf("failed to assign admin role: %v", err)
	}
	err = repo.AssignRoleToUser(ctx, userID, leaderRole.ID)
	if err != nil {
		t.Fatalf("failed to assign leader role: %v", err)
	}

	// 5. Verify User Roles
	roles, err := repo.GetUserRoles(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get user roles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}

	// 6. Verify User Permissions Resolution (de-duplicated)
	perms, err := repo.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get user permissions: %v", err)
	}

	// Admin has 2, Leader has 1 (which overlaps). De-duplicated count must be 2.
	if len(perms) != 2 {
		t.Errorf("expected 2 unique permissions, got %d", len(perms))
	}

	hasCreate := false
	hasSignUp := false
	for _, p := range perms {
		if p.Name == "Create Event" {
			hasCreate = true
		}
		if p.Name == "Sign up for Event" {
			hasSignUp = true
		}
	}

	if !hasCreate || !hasSignUp {
		t.Errorf("missing expected permissions in resolved list (create: %t, signup: %t)", hasCreate, hasSignUp)
	}
}
