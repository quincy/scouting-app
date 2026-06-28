package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupAdminRolesTest(t *testing.T) (*AdminHandler, *auth.AuthService, *sql.DB, *profile.Profile) {
	t.Helper()

	db := testhelper.StartDB()
	store := postgres.NewStore(db)

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()
	if err := auth.SeedRoles(ctx, store.RBAC); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}

	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	handler := NewAdminHandler(store.Profile, store.ParentYouthLink, store.RBAC, authService)

	return handler, authService, db, adminProfile
}

func TestAdminRoles_GetRendersPage(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RolesPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected page to contain admin name, got:\n%s", body)
	}
	if !strings.Contains(body, "admin") {
		t.Errorf("expected page to show 'admin' role, got:\n%s", body)
	}
}

func TestAdminRoles_GetShowsClaimedUsers(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "User",
		Email:      "other@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &otherUser.ID,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected admin user in list, got:\n%s", body)
	}
	if !strings.Contains(body, "Other User") {
		t.Errorf("expected other user in list, got:\n%s", body)
	}
}

func TestAdminRoles_ShowsUnclaimedProfiles(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)

	unclaimed := &profile.Profile{
		FirstName:  "Unclaimed",
		LastName:   "User",
		Email:      "unclaimed@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), unclaimed); err != nil {
		t.Fatalf("Create unclaimed profile: %v", err)
	}

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Unclaimed User") {
		t.Errorf("expected unclaimed profile to be shown, got:\n%s", body)
	}
	if !strings.Contains(body, "not registered") {
		t.Errorf("expected unclaimed profile to show 'not registered', got:\n%s", body)
	}
}

func TestAdminRoles_GrantAdmin(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "User",
		Email:      "other@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &otherUser.ID,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	roles, err := store.RBAC.GetUserRoles(ctx, otherUser.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected other user to have no roles, got %d", len(roles))
	}

	path := "/admin/roles/" + otherUser.ID + "/grant-admin"
	req := familyConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err = store.RBAC.GetUserRoles(ctx, otherUser.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	hasAdmin := false
	for _, role := range roles {
		if role.Name == "admin" {
			hasAdmin = true
			break
		}
	}
	if !hasAdmin {
		t.Error("expected other user to have 'admin' role after grant")
	}
}

func TestAdminRoles_RemoveAdmin(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "User",
		Email:      "other@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &otherUser.ID,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	adminRole, err := store.RBAC.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}
	if err := store.RBAC.AssignRoleToUser(ctx, otherUser.ID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	path := "/admin/roles/" + otherUser.ID + "/remove-admin"
	req := familyConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RemoveAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err := store.RBAC.GetUserRoles(ctx, otherUser.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	hasAdmin := false
	for _, role := range roles {
		if role.Name == "admin" {
			hasAdmin = true
			break
		}
	}
	if hasAdmin {
		t.Error("expected other user to NOT have 'admin' role after removal")
	}
}

func TestAdminRoles_GrantSelfAdminNoop(t *testing.T) {
	handler, authService, db, adminProfile := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)

	adminUserID := *adminProfile.UserID
	path := "/admin/roles/" + adminUserID + "/grant-admin"
	req := familyConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GrantAdmin self returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err := store.RBAC.GetUserRoles(t.Context(), adminUserID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	count := 0
	for _, role := range roles {
		if role.Name == "admin" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 admin role, got %d", count)
	}
}

func TestAdminRoles_GetShowsAdminCount(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Total: 1 profile") {
		t.Errorf("expected total count of 1, got:\n%s", body)
	}
}
