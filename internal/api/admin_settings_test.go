package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupSettingsTest(t *testing.T) (*SettingsHandler, *auth.AuthService, *postgres.Store) {
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

	appCfg := appconfig.NewInMemoryRepository()
	handler := NewSettingsHandler(appCfg)

	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	return handler, authService, store
}

func TestSettingsPage_Defaults(t *testing.T) {
	handler, _, _ := setupSettingsTest(t)

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	w := httptest.NewRecorder()
	handler.SettingsPage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Troop") {
		t.Errorf("expected default unit type 'Troop' in response, got: %s", body)
	}
	if !strings.Contains(body, "America/New_York") {
		t.Errorf("expected default timezone 'America/New_York' in response")
	}
}

func TestSettingsPage_ShowsStoredValues(t *testing.T) {
	_, authService, store := setupSettingsTest(t)
	ctx := t.Context()

	_, adminProfile := seedAdminUser(t, store, &auth.MockHasher{}, ctx)
	appCfg := appconfig.NewInMemoryRepository()
	appCfg.Set(ctx, appconfig.KeyUnitType, "Pack")
	appCfg.Set(ctx, appconfig.KeyUnitNumber, "123")
	appCfg.Set(ctx, appconfig.KeyScoutbookOrgGUID, "test-guid-456")
	appCfg.Set(ctx, appconfig.KeyDefaultTimezone, "America/Chicago")

	handler2 := NewSettingsHandler(appCfg)

	req := loggedInRequest(t, authService, "GET", "/admin/settings")
	w := httptest.NewRecorder()
	handler2.SettingsPage(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Pack") {
		t.Errorf("expected 'Pack' as unit type, got: %s", body)
	}
	if !strings.Contains(body, "123") {
		t.Errorf("expected '123' as unit number")
	}
	if !strings.Contains(body, "test-guid-456") {
		t.Errorf("expected 'test-guid-456' as org GUID")
	}
	if !strings.Contains(body, "America/Chicago") {
		t.Errorf("expected 'America/Chicago' as timezone")
	}
	_ = adminProfile
}

func TestSettingsSave_SavesValues(t *testing.T) {
	handler, _, _ := setupSettingsTest(t)
	ctx := t.Context()

	form := strings.NewReader("unit_type=Pack&unit_number=456&org_guid=guid-789&timezone=America/Denver")
	req := httptest.NewRequest("POST", "/admin/settings", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.SettingsSave(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Settings saved successfully!") {
		t.Errorf("expected success flash message, got: %s", body)
	}
	if !strings.Contains(body, "Pack") {
		t.Errorf("expected 'Pack' in response")
	}
	if !strings.Contains(body, "456") {
		t.Errorf("expected '456' in response")
	}

	unitType, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyUnitType)
	if unitType != "Pack" {
		t.Errorf("expected stored unit type 'Pack', got %q", unitType)
	}
	unitNumber, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyUnitNumber)
	if unitNumber != "456" {
		t.Errorf("expected stored unit number '456', got %q", unitNumber)
	}
	orgGUID, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyScoutbookOrgGUID)
	if orgGUID != "guid-789" {
		t.Errorf("expected stored org GUID 'guid-789', got %q", orgGUID)
	}
	timezone, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyDefaultTimezone)
	if timezone != "America/Denver" {
		t.Errorf("expected stored timezone 'America/Denver', got %q", timezone)
	}
}

func TestSettingsSave_DefaultsForEmptyFields(t *testing.T) {
	handler, _, _ := setupSettingsTest(t)
	ctx := t.Context()

	form := strings.NewReader("unit_type=&unit_number=&org_guid=&timezone=")
	req := httptest.NewRequest("POST", "/admin/settings", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.SettingsSave(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	unitType, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyUnitType)
	if unitType != "Troop" {
		t.Errorf("expected default unit type 'Troop', got %q", unitType)
	}
	timezone, _ := handler.appConfigRepo.Get(ctx, appconfig.KeyDefaultTimezone)
	if timezone != "America/New_York" {
		t.Errorf("expected default timezone 'America/New_York', got %q", timezone)
	}
}

func TestSettingsPage_HtmxPartial(t *testing.T) {
	handler, _, _ := setupSettingsTest(t)

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	handler.SettingsPage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if strings.Contains(body, "admin_layout.html") {
		t.Errorf("HTMX partial should not include admin layout, got: %s", body)
	}
}
