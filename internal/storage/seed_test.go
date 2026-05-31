package storage

import (
	"context"
	"testing"

	"scout-app/internal/domain"
	"scout-app/internal/storage/mock"
)

func TestSeedRoles_AdminHasAllPermissions(t *testing.T) {
	rbac := mock.NewRBACRepository()
	ctx := context.Background()

	if err := rbac.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles failed: %v", err)
	}

	// Assign admin role to a user and check permissions
	adminRole, err := rbac.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName(admin) failed: %v", err)
	}

	userID := "test-user"
	if err := rbac.AssignRoleToUser(ctx, userID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	perms, err := rbac.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}

	expected := map[string]bool{
		"event:create":   false,
		"event:view":     false,
		"event:signup":   false,
		"event:withdraw": false,
	}
	for _, p := range perms {
		expected[p.Name] = true
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected permission %q to be assigned to admin role, but it was missing", name)
		}
	}
}

func TestSeedRoles_ScoutHasCorrectPermissions(t *testing.T) {
	rbac := mock.NewRBACRepository()
	ctx := context.Background()

	if err := rbac.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles failed: %v", err)
	}

	scoutRole, err := rbac.GetRoleByName(ctx, "scout")
	if err != nil {
		t.Fatalf("GetRoleByName(scout) failed: %v", err)
	}

	userID := "scout-user"
	if err := rbac.AssignRoleToUser(ctx, userID, scoutRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	perms, err := rbac.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}

	permNames := make(map[string]bool)
	for _, p := range perms {
		permNames[p.Name] = true
	}

	// Scouts should NOT have event:create
	if permNames["event:create"] {
		t.Error("scout should not have event:create permission")
	}
	if !permNames["event:view"] {
		t.Error("scout should have event:view permission")
	}
	if !permNames["event:signup"] {
		t.Error("scout should have event:signup permission")
	}
	if !permNames["event:withdraw"] {
		t.Error("scout should have event:withdraw permission")
	}
}

// Ensure the domain.SeedRoles function works via the interface too
func TestDomainSeedRoles(t *testing.T) {
	rbac := mock.NewRBACRepository()
	ctx := context.Background()

	if err := domain.SeedRoles(ctx, rbac); err != nil {
		t.Fatalf("domain.SeedRoles failed: %v", err)
	}

	adminRole, err := rbac.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName(admin) failed: %v", err)
	}

	userID := "domain-test-user"
	if err := rbac.AssignRoleToUser(ctx, userID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	perms, err := rbac.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}

	if len(perms) != 4 {
		t.Errorf("expected 4 permissions for admin, got %d: %v", len(perms), perms)
	}
}
