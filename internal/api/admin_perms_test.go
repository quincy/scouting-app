package api

import (
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

func TestComputeAdminPerms_AdminUserHasAllPerms(t *testing.T) {
	db := testhelper.StartDB()
	store := postgres.NewStore(db)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	hasher := &auth.MockHasher{}
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, auth.NewCookieStore("test-secret-key"))

	ctx := t.Context()
	adminUser, _ := seedAdminUser(t, store, hasher, ctx)

	_ = authService

	perms := computeAdminPerms(ctx, store.RBAC, adminUser.ID)

	if !perms.CanAccessRoster {
		t.Error("expected admin to have CanAccessRoster")
	}
	if !perms.CanAccessConnections {
		t.Error("expected admin to have CanAccessConnections")
	}
	if !perms.CanAccessSync {
		t.Error("expected admin to have CanAccessSync")
	}
	if !perms.CanAccessRBAC {
		t.Error("expected admin to have CanAccessRBAC")
	}
	if !perms.CanAccessSettings {
		t.Error("expected admin to have CanAccessSettings")
	}
}

func TestAdminPerms_FirstAvailableAdminUser(t *testing.T) {
	perms := AdminPerms{
		CanAccessRoster:      true,
		CanAccessConnections: true,
		CanAccessSync:        true,
		CanAccessRBAC:        true,
		CanAccessSettings:    true,
	}

	redirect := perms.FirstAvailable()
	if redirect != "/admin/roster" {
		t.Errorf("expected FirstAvailable to be /admin/roster, got %q", redirect)
	}
}

func TestAdminPerms_FirstAvailableOnlySync(t *testing.T) {
	perms := AdminPerms{
		CanAccessSync: true,
	}

	redirect := perms.FirstAvailable()
	if redirect != "/admin/sync" {
		t.Errorf("expected FirstAvailable to be /admin/sync, got %q", redirect)
	}
}

func TestAdminPerms_FirstAvailableNoPerms(t *testing.T) {
	perms := AdminPerms{}

	redirect := perms.FirstAvailable()
	if redirect != "" {
		t.Errorf("expected FirstAvailable to be empty, got %q", redirect)
	}
}

func TestAdminPage_SignedInAsAdminRedirectsToRoster(t *testing.T) {
	db := testhelper.StartDB()
	store := postgres.NewStore(db)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()
	seedAdminUser(t, store, hasher, ctx)

	handler := NewAdminHandler(store.Profile, store.ParentYouthLink, store.RBAC, authService)

	req := loggedInRequest(t, authService, "GET", "/admin")
	rr := httptest.NewRecorder()

	handler.AdminPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	loc := rr.Header().Get("Location")
	if loc != "/admin/roster" {
		t.Errorf("expected redirect to /admin/roster, got %q", loc)
	}
}

func TestAdminPage_SignedInWithNoAdminPermsReturnsForbidden(t *testing.T) {
	db := testhelper.StartDB()
	store := postgres.NewStore(db)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()

	// Create a user with a role that has no admin perms
	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &user.User{
		Email:        "scout@test.com",
		PasswordHash: hash,
	}
	if err := store.User.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	profile := &profile.Profile{
		FirstName:  "Test",
		LastName:   "Scout",
		Email:      "scout@test.com",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
		UserID:     &u.ID,
	}
	if err := store.Profile.Create(ctx, profile); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	role, err := store.RBAC.GetRoleByName(ctx, "Scouts BSA")
	if err != nil {
		t.Fatalf("GetRoleByName: %v", err)
	}
	if err := store.RBAC.AssignRoleToUser(ctx, u.ID, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	// Login as this user
	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=scout@test.com&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)
	req := httptest.NewRequest("GET", "/admin", nil)
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}

	handler := NewAdminHandler(store.Profile, store.ParentYouthLink, store.RBAC, authService)
	rr := httptest.NewRecorder()
	handler.AdminPage(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}
