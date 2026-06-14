package storage

import (
	"context"
	"testing"

	"scout-app/internal/domain/auth"
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

	scoutRole, err := rbac.GetRoleByName(ctx, "Scouts BSA")
	if err != nil {
		t.Fatalf("GetRoleByName(Scouts BSA) failed: %v", err)
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

func TestSeedRoles_ScoutbookPositionRolesHaveNoPermissions(t *testing.T) {
	rbac := mock.NewRBACRepository()
	ctx := context.Background()

	if err := rbac.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles failed: %v", err)
	}

	positionRoles := []string{
		"Assistant Patrol Leader",
		"Assistant Senior Patrol Leader",
		"Chaplain Aide",
		"Chartered Organization Rep.",
		"Committee Chairman",
		"Committee Member",
		"Den Chief",
		"Executive Officer",
		"Historian",
		"Librarian",
		"Life-to-Eagle Coordinator",
		"OA Unit Representative",
		"Outdoor Ethics Guide",
		"Patrol Admin",
		"Patrol Leader",
		"Quartermaster",
		"Scribe",
		"Senior Patrol Leader",
		"Troop Admin",
		"Troop Guide",
		"Unit Advancement Chair",
		"Unit College Scouter Reserve",
		"Unit Outdoors / Activities Chair",
		"Unit Public Relations Chair",
		"Unit Scouter Reserve",
		"Unit Training Chair",
		"Unit Treasurer",
		"Webmaster",
		"Youth Protection Champion",
	}

	userID := "position-role-user"
	for _, roleName := range positionRoles {
		role, err := rbac.GetRoleByName(ctx, roleName)
		if err != nil {
			t.Fatalf("GetRoleByName(%q) failed: %v", roleName, err)
		}
		if err := rbac.AssignRoleToUser(ctx, userID, role.ID); err != nil {
			t.Fatalf("AssignRoleToUser(%q) failed: %v", roleName, err)
		}
	}

	perms, err := rbac.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions for Scoutbook position roles, got %d: %v", len(perms), perms)
	}
}

// Ensure the domain.SeedRoles function works via the interface too
func TestDomainSeedRoles(t *testing.T) {
	rbac := mock.NewRBACRepository()
	ctx := context.Background()

	if err := auth.SeedRoles(ctx, rbac); err != nil {
		t.Fatalf("auth.SeedRoles failed: %v", err)
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
