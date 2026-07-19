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

func setupRosterTest(t *testing.T) (*AdminHandler, *auth.AuthService, *sql.DB, *profile.Profile) {
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

func rosterLoggedInRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
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

func TestToggleProfileStatus_CyclesToInactive(t *testing.T) {
	handler, authService, db, _ := setupRosterTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	p := &profile.Profile{
		FirstName:  "Test",
		LastName:   "User",
		Email:      "test@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	path := "/admin/roster/" + p.ID + "/toggle-status?id=" + p.ID
	req := rosterLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ToggleProfileStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleProfileStatus returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.Profile.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != profile.StatusInactive {
		t.Errorf("expected status %q, got %q", profile.StatusInactive, updated.Status)
	}
}

func TestToggleProfileStatus_CyclesToDisabled(t *testing.T) {
	handler, authService, db, _ := setupRosterTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	p := &profile.Profile{
		FirstName:  "Test",
		LastName:   "User",
		Email:      "test@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusInactive,
	}
	if err := store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	path := "/admin/roster/" + p.ID + "/toggle-status?id=" + p.ID
	req := rosterLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ToggleProfileStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleProfileStatus returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.Profile.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != profile.StatusDisabled {
		t.Errorf("expected status %q, got %q", profile.StatusDisabled, updated.Status)
	}
}

func TestToggleProfileStatus_CyclesToActive(t *testing.T) {
	handler, authService, db, _ := setupRosterTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := t.Context()

	p := &profile.Profile{
		FirstName:  "Test",
		LastName:   "User",
		Email:      "test@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusDisabled,
	}
	if err := store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	path := "/admin/roster/" + p.ID + "/toggle-status?id=" + p.ID
	req := rosterLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ToggleProfileStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleProfileStatus returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.Profile.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != profile.StatusActive {
		t.Errorf("expected status %q, got %q", profile.StatusActive, updated.Status)
	}
}

func TestToggleProfileStatus_NotFound(t *testing.T) {
	handler, authService, db, _ := setupRosterTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	path := "/admin/roster/nonexistent/toggle-status?id=nonexistent"
	req := rosterLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ToggleProfileStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ToggleProfileStatus returned status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestToggleProfileStatus_Unauthenticated(t *testing.T) {
	handler, _, db, _ := setupRosterTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := httptest.NewRequest("POST", "/admin/roster/some-id/toggle-status?id=some-id", nil)
	rr := httptest.NewRecorder()

	handler.ToggleProfileStatus(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ToggleProfileStatus returned status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
