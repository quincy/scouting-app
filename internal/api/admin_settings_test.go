package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/storage/mock"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupSettingsTest(t *testing.T) (*SettingsHandler, *auth.AuthService, *postgres.Store, *mock.EmailService) {
	t.Helper()

	db := testhelper.StartDB()
	store := postgres.NewStore(db)

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	appCfg := appconfig.NewInMemoryRepository()
	emailSvc := mock.NewEmailService()
	handler := NewSettingsHandler(appCfg, emailSvc, authService, store.Profile, store.RBAC)

	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	return handler, authService, store, emailSvc
}

func TestSettingsPage_Defaults(t *testing.T) {
	handler, _, _, _ := setupSettingsTest(t)

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	w := httptest.NewRecorder()
	handler.SettingsPage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Troop") {
		t.Errorf("expected default unit type 'Troop' in response")
	}
	if !strings.Contains(body, "America/New_York") {
		t.Errorf("expected default timezone 'America/New_York' in response")
	}
}

func TestSettingsPage_ShowsStoredValues(t *testing.T) {
	_, authService, store, _ := setupSettingsTest(t)
	ctx := t.Context()

	_, adminProfile := seedAdminUser(t, store, &auth.MockHasher{}, ctx)
	appCfg := appconfig.NewInMemoryRepository()
	appCfg.Set(ctx, appconfig.KeyUnitType, "Pack")
	appCfg.Set(ctx, appconfig.KeyUnitNumber, "123")
	appCfg.Set(ctx, appconfig.KeyScoutbookOrgGUID, "test-guid-456")
	appCfg.Set(ctx, appconfig.KeyDefaultTimezone, "America/Chicago")
	appCfg.Set(ctx, appconfig.KeySMTPHost, "smtp.example.com")
	appCfg.Set(ctx, appconfig.KeySMTPPort, "587")
	appCfg.Set(ctx, appconfig.KeySMTPUser, "user@example.com")
	appCfg.Set(ctx, appconfig.KeySMTPFrom, "from@example.com")

	emailSvc := mock.NewEmailService()
	handler2 := NewSettingsHandler(appCfg, emailSvc, authService, store.Profile, store.RBAC)

	req := loggedInRequest(t, authService, "GET", "/admin/settings")
	w := httptest.NewRecorder()
	handler2.SettingsPage(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Pack") {
		t.Errorf("expected 'Pack' as unit type")
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
	if !strings.Contains(body, "smtp.example.com") {
		t.Errorf("expected 'smtp.example.com' as SMTP host")
	}
	if !strings.Contains(body, "587") {
		t.Errorf("expected '587' as SMTP port")
	}
	if !strings.Contains(body, "user@example.com") {
		t.Errorf("expected 'user@example.com' as SMTP user")
	}
	if !strings.Contains(body, "from@example.com") {
		t.Errorf("expected 'from@example.com' as SMTP from")
	}
	_ = adminProfile
}

func TestSettingsSave_SavesValues(t *testing.T) {
	handler, _, _, _ := setupSettingsTest(t)
	ctx := t.Context()

	form := strings.NewReader("unit_type=Pack&unit_number=456&org_guid=guid-789&timezone=America/Denver&smtp_host=smtp.test.com&smtp_port=587&smtp_user=testuser&smtp_pass=secret&smtp_from=test@test.com")
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
		t.Errorf("expected success flash message")
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
	smtpHost, _ := handler.appConfigRepo.Get(ctx, appconfig.KeySMTPHost)
	if smtpHost != "smtp.test.com" {
		t.Errorf("expected stored SMTP host 'smtp.test.com', got %q", smtpHost)
	}
	smtpPass, _ := handler.appConfigRepo.Get(ctx, appconfig.KeySMTPPass)
	if smtpPass != "secret" {
		t.Errorf("expected stored SMTP pass 'secret', got %q", smtpPass)
	}
}

func TestSettingsSave_DefaultsForEmptyFields(t *testing.T) {
	handler, _, _, _ := setupSettingsTest(t)
	ctx := t.Context()

	form := strings.NewReader("unit_type=&unit_number=&org_guid=&timezone=&smtp_host=&smtp_port=&smtp_user=&smtp_pass=&smtp_from=")
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

func TestSettingsSave_BlankPasswordKeepsExisting(t *testing.T) {
	handler, _, _, _ := setupSettingsTest(t)
	ctx := t.Context()

	handler.appConfigRepo.Set(ctx, appconfig.KeySMTPPass, "existing-secret")

	form := strings.NewReader("unit_type=Troop&unit_number=&org_guid=&timezone=America/New_York&smtp_host=&smtp_port=&smtp_user=&smtp_pass=&smtp_from=")
	req := httptest.NewRequest("POST", "/admin/settings", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.SettingsSave(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	smtpPass, _ := handler.appConfigRepo.Get(ctx, appconfig.KeySMTPPass)
	if smtpPass != "existing-secret" {
		t.Errorf("expected SMTP pass to be preserved as 'existing-secret', got %q", smtpPass)
	}
}

func TestSettingsPage_HtmxPartial(t *testing.T) {
	handler, _, _, _ := setupSettingsTest(t)

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
		t.Errorf("HTMX partial should not include admin layout")
	}
}

func TestTestEmail_SendsToAdmin(t *testing.T) {
	_, authService, store, emailSvc := setupSettingsTest(t)
	ctx := t.Context()

	_, _ = seedAdminUser(t, store, &auth.MockHasher{}, ctx)

	appCfg := appconfig.NewInMemoryRepository()
	handler := NewSettingsHandler(appCfg, emailSvc, authService, store.Profile, store.RBAC)

	req := loggedInRequest(t, authService, "POST", "/admin/settings/test-email")
	w := httptest.NewRecorder()
	handler.TestEmail(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test email sent") {
		t.Errorf("expected success message, got: %s", body)
	}

	t.Logf("SentNotifications: %+v", emailSvc.SentNotifications)
	if len(emailSvc.SentNotifications) == 0 {
		t.Fatal("no notifications were sent")
	}
	if len(emailSvc.SentNotifications[0].To) == 0 {
		t.Fatalf("To[0] is empty, all To: %#v", emailSvc.SentNotifications[0].To)
	}
	if emailSvc.SentNotifications[0].To[0] != "admin@scout.local" {
		t.Errorf("expected email to admin@scout.local, got %#v", emailSvc.SentNotifications[0].To)
	}
}
