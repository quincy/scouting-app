package api

import (
	"context"
	"os"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.StartDB()
	os.Exit(m.Run())
}

// seedAdminUser creates an admin user + profile + assigns admin role.
// Returns the adminUser, adminProfile, and adminRole.
func seedAdminUser(t testing.TB, store *postgres.Store, hasher auth.Hasher, ctx context.Context) (*user.User, *profile.Profile) {
	t.Helper()

	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	adminUser := &user.User{
		Email:        "admin@scout.local",
		PasswordHash: hash,
	}
	if err := store.User.Create(ctx, adminUser); err != nil {
		t.Fatalf("Create admin user: %v", err)
	}

	adminProfile := &profile.Profile{
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := store.Profile.Create(ctx, adminProfile); err != nil {
		t.Fatalf("Create admin profile: %v", err)
	}

	adminRole, err := store.RBAC.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}

	if err := store.RBAC.AssignRoleToUser(ctx, adminUser.ID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser admin: %v", err)
	}

	return adminUser, adminProfile
}
