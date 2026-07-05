package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
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
	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{"id": r.URL.Query().Get("id")}
	})

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
	if !strings.Contains(body, "admin") {
		t.Errorf("expected page to show admin role, got:\n%s", body)
	}
	if !strings.Contains(body, "event:view") {
		t.Errorf("expected page to show event:view permission, got:\n%s", body)
	}
}

func TestAdminRoles_ShowsAllRoles(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Scoutmaster") {
		t.Errorf("expected page to show Scoutmaster role, got:\n%s", body)
	}
	if !strings.Contains(body, "Scouts BSA") {
		t.Errorf("expected page to show Scouts BSA role, got:\n%s", body)
	}
	if !strings.Contains(body, "parent") {
		t.Errorf("expected page to show parent role, got:\n%s", body)
	}
}

func TestAdminRoles_SplitsAdultAndYouthRoles(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Adult Roles") {
		t.Errorf("expected page to show Adult Roles heading, got:\n%s", body)
	}
	if !strings.Contains(body, "Youth Roles") {
		t.Errorf("expected page to show Youth Roles heading, got:\n%s", body)
	}
}

func TestAdminRoles_AdultRolesInAdultSection(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()

	adultIdx := strings.Index(body, "Adult Roles")
	youthIdx := strings.Index(body, "Youth Roles")
	if adultIdx < 0 || youthIdx < 0 {
		t.Fatal("expected both Adult Roles and Youth Roles headings")
	}

	adminIdx := strings.Index(body, ">admin<")
	if adminIdx < 0 || adminIdx > youthIdx {
		t.Errorf("expected admin role to appear before Youth Roles section (adminIdx=%d, youthIdx=%d)", adminIdx, youthIdx)
	}

	scoutsIdx := strings.Index(body, "Scouts BSA")
	if scoutsIdx < 0 || scoutsIdx < youthIdx {
		t.Errorf("expected Scouts BSA role to appear in Youth Roles section (scoutsIdx=%d, youthIdx=%d)", scoutsIdx, youthIdx)
	}
}

func TestAdminRoles_AdminRoleHasAdminRbac(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "admin:rbac") {
		t.Errorf("expected admin role to have admin:rbac permission, got:\n%s", body)
	}
}

func TestAdminRoles_ShowsUserCount(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/admin/roles", "")
	rr := httptest.NewRecorder()

	handler.RolesPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, ">1<") {
		t.Errorf("expected admin row to show user count of 1, got:\n%s", body)
	}
}

func TestAdminRoles_EditModalShowsPermissions(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	adminRole, err := store.RBAC.GetRoleByName(t.Context(), "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}

	path := "/admin/roles/" + adminRole.ID + "/edit?id=" + adminRole.ID
	req := familyConnLoggedInRequest(t, authService, "GET", path, "")
	rr := httptest.NewRecorder()

	handler.RolesEditModal(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RolesEditModal returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "admin:rbac") {
		t.Errorf("expected modal to show admin:rbac permission, got:\n%s", body)
	}
	if !strings.Contains(body, "event:view") {
		t.Errorf("expected modal to show event:view permission, got:\n%s", body)
	}
	if !strings.Contains(body, "checked") {
		t.Errorf("expected admin permissions to be checked, got:\n%s", body)
	}
}

func TestAdminRoles_EditModalAdminRbacIsDisabled(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	adminRole, err := store.RBAC.GetRoleByName(t.Context(), "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}

	path := "/admin/roles/" + adminRole.ID + "/edit?id=" + adminRole.ID
	req := familyConnLoggedInRequest(t, authService, "GET", path, "")
	rr := httptest.NewRecorder()

	handler.RolesEditModal(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `disabled`) {
		t.Errorf("expected admin:rbac checkbox to be disabled, got:\n%s", body)
	}
}

func TestAdminRoles_SavePermissions(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	scoutRole, err := store.RBAC.GetRoleByName(ctx, "Scouts BSA")
	if err != nil {
		t.Fatalf("GetRoleByName Scouts BSA: %v", err)
	}

	allPerms, err := store.RBAC.ListAllPermissions(ctx)
	if err != nil {
		t.Fatalf("ListAllPermissions: %v", err)
	}

	var eventViewPermID string
	for _, p := range allPerms {
		if p.Name == "event:view" {
			eventViewPermID = p.ID
			break
		}
	}
	if eventViewPermID == "" {
		t.Fatal("event:view permission not found")
	}

	path := "/admin/roles/" + scoutRole.ID + "/permissions?id=" + scoutRole.ID
	body := "permissions=" + eventViewPermID
	req := familyConnLoggedInRequest(t, authService, "POST", path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.RolesSavePermissions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RolesSavePermissions returned status %d, want %d", rr.Code, http.StatusOK)
	}

	rolePerms, err := store.RBAC.GetRolePermissions(ctx, scoutRole.ID)
	if err != nil {
		t.Fatalf("GetRolePermissions: %v", err)
	}

	if len(rolePerms) != 1 {
		t.Errorf("expected 1 permission for Scouts BSA after save, got %d", len(rolePerms))
	}
	if len(rolePerms) > 0 && rolePerms[0].Name != "event:view" {
		t.Errorf("expected permission to be event:view, got %s", rolePerms[0].Name)
	}
}

func TestAdminRoles_NonAdminRoleDoesNotHaveAdminRbacOptionally(t *testing.T) {
	handler, authService, db, _ := setupAdminRolesTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	scoutRole, err := store.RBAC.GetRoleByName(t.Context(), "Scouts BSA")
	if err != nil {
		t.Fatalf("GetRoleByName Scouts BSA: %v", err)
	}

	path := "/admin/roles/" + scoutRole.ID + "/edit?id=" + scoutRole.ID
	req := familyConnLoggedInRequest(t, authService, "GET", path, "")
	rr := httptest.NewRecorder()

	handler.RolesEditModal(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, `(required)`) {
		t.Errorf("expected non-admin role not to have required permission label, got:\n%s", body)
	}
}
