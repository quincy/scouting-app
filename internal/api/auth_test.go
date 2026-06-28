package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupAuthTest(t *testing.T) (*AuthHandler, *auth.AuthService, *sql.DB) {
	t.Helper()
	db := testhelper.StartDB()
	store := postgres.NewStore(db)
	hasher := &auth.MockHasher{}

	ctx := t.Context()
	if err := auth.SeedRoles(ctx, store.RBAC); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}

	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)
	authHandler := NewAuthHandler(authService)
	return authHandler, authService, db
}

func TestAuthHandler_LoginPage(t *testing.T) {
	handler, _, _ := setupAuthTest(t)

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()

	handler.LoginPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("LoginPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Sign In") {
		t.Errorf("expected page to contain 'Sign In', got:\n%s", body)
	}
	if !strings.Contains(body, "type=\"email\"") {
		t.Errorf("expected email input, got:\n%s", body)
	}
	if !strings.Contains(body, "type=\"password\"") {
		t.Errorf("expected password input, got:\n%s", body)
	}
	if !strings.Contains(body, "method=\"POST\"") || !strings.Contains(body, "/login") {
		t.Errorf("expected login form POST to /login, got:\n%s", body)
	}
}

func TestAuthHandler_LoginPage_WithError(t *testing.T) {
	handler, _, _ := setupAuthTest(t)

	req := httptest.NewRequest("GET", "/login?error=Invalid+credentials", nil)
	rr := httptest.NewRecorder()

	handler.LoginPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Invalid credentials") {
		t.Errorf("expected error message to be shown, got:\n%s", body)
	}
}

func TestAuthHandler_Login_ValidCredentials(t *testing.T) {
	handler, _, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	ctx := t.Context()
	store := postgres.NewStore(db)
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Login returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/events" {
		t.Errorf("expected redirect to /events, got %q", location)
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	handler, _, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	ctx := t.Context()
	store := postgres.NewStore(db)
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Login returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=Invalid+credentials") {
		t.Errorf("expected redirect with error, got %q", location)
	}
}

func TestAuthHandler_Login_WithRedirect(t *testing.T) {
	handler, _, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	ctx := t.Context()
	store := postgres.NewStore(db)
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	req := httptest.NewRequest("POST", "/login?redirect=/events/upcoming", strings.NewReader("email=admin@scout.local&password=password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Login returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/events/upcoming" {
		t.Errorf("expected redirect to /events/upcoming, got %q", location)
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	handler, _, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	ctx := t.Context()
	store := postgres.NewStore(db)
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	handler.Login(loginRR, loginReq)

	cookies := loginRR.Result().Cookies()

	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	for _, c := range cookies {
		logoutReq.AddCookie(c)
	}
	logoutRR := httptest.NewRecorder()
	handler.Logout(logoutRR, logoutReq)

	if logoutRR.Code != http.StatusFound {
		t.Errorf("Logout returned status %d, want 302 Found", logoutRR.Code)
	}

	location := logoutRR.Header().Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %q", location)
	}
}

func TestRequireAuth_NoSession(t *testing.T) {
	_, authService, _ := setupAuthTest(t)

	var called bool
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	wrapped := RequireAuth(authService, protected)

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("RequireAuth returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("expected redirect to /login, got %q", location)
	}

	if called {
		t.Error("protected handler should not have been called")
	}
}

func TestRequireAuth_ValidSession(t *testing.T) {
	handler, authService, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	ctx := t.Context()
	store := postgres.NewStore(db)
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	handler.Login(loginRR, loginReq)
	cookies := loginRR.Result().Cookies()

	var called bool
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	wrapped := RequireAuth(authService, protected)

	req := httptest.NewRequest("GET", "/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if !called {
		t.Error("protected handler should have been called when authenticated")
	}
}

func TestRequirePermission_UserLacksPermission(t *testing.T) {
	handler, authService, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)

	ctx := t.Context()
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	handler.Login(loginRR, loginReq)
	cookies := loginRR.Result().Cookies()

	var called bool
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	wrapped := RequirePermission(authService, store.RBAC, "event:admin", protected)

	req := httptest.NewRequest("GET", "/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("RequirePermission returned status %d, want 403", rr.Code)
	}

	if called {
		t.Error("protected handler should not have been called without permission")
	}
}

func TestRequirePermission_UserHasPermission(t *testing.T) {
	handler, authService, db := setupAuthTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)

	ctx := t.Context()
	seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	handler.Login(loginRR, loginReq)
	cookies := loginRR.Result().Cookies()

	var called bool
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	wrapped := RequirePermission(authService, store.RBAC, "event:view", protected)

	req := httptest.NewRequest("GET", "/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if !called {
		t.Error("protected handler should have been called with correct permission")
	}
}
