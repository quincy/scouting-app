package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/mock"
)

func setupAdminRolesTest(t *testing.T) (*AdminHandler, *auth.AuthService, *mock.UserRepository, *mock.ProfileRepository, *mock.RBACRepository, *profile.Profile) {
	t.Helper()

	userRepo := mock.NewUserRepository()
	profileRepo := mock.NewProfileRepository()
	rbacRepo := mock.NewRBACRepository()

	hasher := &auth.MockHasher{}
	store := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(userRepo, rbacRepo, hasher, store)

	ctx := t.Context()
	if err := rbacRepo.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}
	if err := authService.SeedAdminUser(ctx); err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}

	adminUser, err := userRepo.GetByEmail(ctx, "admin@scout.local")
	if err != nil {
		t.Fatalf("GetByEmail admin: %v", err)
	}

	adminProfile := &profile.Profile{
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := profileRepo.Create(ctx, adminProfile); err != nil {
		t.Fatalf("Create admin profile: %v", err)
	}

	handler := NewAdminHandler(profileRepo, mock.NewParentYouthLinkRepository(), rbacRepo, authService)

	return handler, authService, userRepo, profileRepo, rbacRepo, adminProfile
}

func adminRolesLoggedInRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
	t.Helper()

	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)

	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestAdminRoles_GetRendersPage(t *testing.T) {
	handler, authService, _, _, _, _ := setupAdminRolesTest(t)

	req := adminRolesLoggedInRequest(t, authService, "GET", "/admin/roles", "")
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
	handler, authService, userRepo, profileRepo, _, _ := setupAdminRolesTest(t)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := userRepo.Create(ctx, otherUser); err != nil {
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
	if err := profileRepo.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	req := adminRolesLoggedInRequest(t, authService, "GET", "/admin/roles", "")
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
	handler, authService, _, profileRepo, _, _ := setupAdminRolesTest(t)

	unclaimed := &profile.Profile{
		FirstName:  "Unclaimed",
		LastName:   "User",
		Email:      "unclaimed@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(t.Context(), unclaimed); err != nil {
		t.Fatalf("Create unclaimed profile: %v", err)
	}

	req := adminRolesLoggedInRequest(t, authService, "GET", "/admin/roles", "")
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
	handler, authService, userRepo, profileRepo, rbacRepo, _ := setupAdminRolesTest(t)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := userRepo.Create(ctx, otherUser); err != nil {
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
	if err := profileRepo.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	roles, err := rbacRepo.GetUserRoles(ctx, otherUser.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected other user to have no roles, got %d", len(roles))
	}

	path := "/admin/roles/" + otherUser.ID + "/grant-admin"
	req := adminRolesLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err = rbacRepo.GetUserRoles(ctx, otherUser.ID)
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
	handler, authService, userRepo, profileRepo, rbacRepo, _ := setupAdminRolesTest(t)
	ctx := t.Context()

	otherUser := &user.User{PasswordHash: "hash"}
	if err := userRepo.Create(ctx, otherUser); err != nil {
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
	if err := profileRepo.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	adminRole, err := rbacRepo.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}
	if err := rbacRepo.AssignRoleToUser(ctx, otherUser.ID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	path := "/admin/roles/" + otherUser.ID + "/remove-admin"
	req := adminRolesLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RemoveAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err := rbacRepo.GetUserRoles(ctx, otherUser.ID)
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
	handler, authService, _, _, rbacRepo, adminProfile := setupAdminRolesTest(t)

	adminUserID := *adminProfile.UserID
	path := "/admin/roles/" + adminUserID + "/grant-admin"
	req := adminRolesLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GrantAdmin self returned status %d, want %d", rr.Code, http.StatusOK)
	}

	roles, err := rbacRepo.GetUserRoles(t.Context(), adminUserID)
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
	handler, authService, _, _, _, _ := setupAdminRolesTest(t)

	req := adminRolesLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Total: 1 profile") {
		t.Errorf("expected total count of 1, got:\n%s", body)
	}
}
