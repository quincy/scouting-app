package storage

import (
	"context"
	"testing"

	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func TestRBACRepository_RolesAndPermissions(t *testing.T) {
	db := testhelper.StartDB()
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	repo := postgres.NewRBACRepository(db)
	ctx := context.Background()

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

	dupRole := &rbac.Role{Name: "Admin"}
	err = repo.CreateRole(ctx, dupRole)
	if err != nil {
		t.Fatalf("expected no error for duplicate role (idempotent), got: %v", err)
	}
	if dupRole.ID != adminRole.ID {
		t.Error("expected duplicate role to get the existing role's ID")
	}

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

	err = repo.LinkPermissionToRole(ctx, adminRole.ID, createEventPerm.ID)
	if err != nil {
		t.Fatalf("failed to link permission to admin: %v", err)
	}
	_ = repo.LinkPermissionToRole(ctx, adminRole.ID, signUpPerm.ID)

	_ = repo.LinkPermissionToRole(ctx, leaderRole.ID, signUpPerm.ID)

	userRepo := postgres.NewUserRepository(db)
	u := &user.User{PasswordHash: "password"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	err = repo.AssignRoleToUser(ctx, u.ID, adminRole.ID)
	if err != nil {
		t.Fatalf("failed to assign admin role: %v", err)
	}
	err = repo.AssignRoleToUser(ctx, u.ID, leaderRole.ID)
	if err != nil {
		t.Fatalf("failed to assign leader role: %v", err)
	}

	roles, err := repo.GetUserRoles(ctx, u.ID)
	if err != nil {
		t.Fatalf("failed to get user roles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}

	perms, err := repo.GetUserPermissions(ctx, u.ID)
	if err != nil {
		t.Fatalf("failed to get user permissions: %v", err)
	}

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
