package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/mock"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupFamilyConnectionsTest(t *testing.T) (*FamilyConnectionsHandler, *auth.AuthService, *sql.DB, *mock.EmailService, *profile.Profile, *profile.Profile) {
	t.Helper()

	db := testhelper.StartDB()
	store := postgres.NewStore(db)

	emailSvc := mock.NewEmailService()

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()
	if err := auth.SeedRoles(ctx, store.RBAC); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}

	_, parentProfile := seedAdminUser(t, store, hasher, ctx)
	parentProfile.BSAID = "PAR001"
	parentProfile.FirstName = "Parent"
	if err := store.Profile.Update(ctx, parentProfile); err != nil {
		t.Fatalf("Update parent profile: %v", err)
	}

	youthProfile := &profile.Profile{
		FirstName:  "Alex",
		LastName:   "Youth",
		BSAID:      "YTH001",
		Email:      "alex.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	handler := NewFamilyConnectionsHandler(store.Profile, store.ParentYouthLink, authService, store.RBAC, emailSvc)

	return handler, authService, db, emailSvc, parentProfile, youthProfile
}

func familyConnLoggedInRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
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

func TestFamilyConnections_GetRendersPage(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/family-connections", "")
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("FamilyConnectionsPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Family Connections") {
		t.Errorf("expected page to contain 'Family Connections', got:\n%s", body)
	}
}

func TestFamilyConnections_GetRedirectsWhenNotLoggedIn(t *testing.T) {
	handler, _, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := httptest.NewRequest("GET", "/family-connections", nil)
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("FamilyConnectionsPage returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}
}

func TestFamilyConnections_GetShowsEmptyState(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/family-connections", "")
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "No family connections yet") {
		t.Errorf("expected empty state message, got:\n%s", body)
	}
}

func TestFamilyConnections_GetShowsFormForAdult(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "GET", "/family-connections", "")
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Add a Connection") {
		t.Errorf("expected form for adult user, got:\n%s", body)
	}
	if !strings.Contains(body, "bsa_id") {
		t.Errorf("expected bsa_id input, got:\n%s", body)
	}
}

func TestFamilyConnections_GetHidesFormForYouth(t *testing.T) {
	_, _, db, _, _, youthProfile := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)

	youthUser := &user.User{
		Email:        "alex.youth@scout.local",
		PasswordHash: "password",
	}
	if err := store.User.Create(t.Context(), youthUser); err != nil {
		t.Fatalf("Create youth user: %v", err)
	}

	youthProfile.UserID = &youthUser.ID
	if err := store.Profile.Update(t.Context(), youthProfile); err != nil {
		t.Fatalf("Update youth profile: %v", err)
	}

	hasher2 := &auth.MockHasher{}
	cookieStore2 := auth.NewCookieStore("test-secret-key")
	authService2 := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher2, cookieStore2)

	handler2 := NewFamilyConnectionsHandler(store.Profile, store.ParentYouthLink, authService2, store.RBAC, mock.NewEmailService())

	authHandler2 := NewAuthHandler(authService2)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=alex.youth@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler2.Login(loginRR, loginReq)

	req := httptest.NewRequest("GET", "/family-connections", nil)
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()

	handler2.FamilyConnectionsPage(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "Add a Connection") {
		t.Errorf("expected no form for youth user, got form")
	}
}

func TestFamilyConnections_GetShowsExistingConnections(t *testing.T) {
	handler, authService, db, _, parentProfile, youthProfile := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parentProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
	}
	if err := postgres.NewStore(db).ParentYouthLink.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	req := familyConnLoggedInRequest(t, authService, "GET", "/family-connections", "")
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Alex Youth") {
		t.Errorf("expected connection to show 'Alex Youth', got:\n%s", body)
	}
	if !strings.Contains(body, "approved") {
		t.Errorf("expected status 'approved', got:\n%s", body)
	}
}

func TestFamilyConnections_PostValidBSAID(t *testing.T) {
	handler, authService, db, _, parentProfile, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=YTH001")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	links, err := postgres.NewStore(db).ParentYouthLink.ListByParent(t.Context(), parentProfile.ID)
	if err != nil {
		t.Fatalf("ListByParent: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Status != parentyouthlink.StatusPending {
		t.Errorf("expected link status %q, got %q", parentyouthlink.StatusPending, links[0].Status)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Request sent") {
		t.Errorf("expected success message 'Request sent', got:\n%s", body)
	}
}

func TestFamilyConnections_PostInvalidBSAID(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=NONEXIST")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "No profile found") {
		t.Errorf("expected error 'No profile found', got:\n%s", body)
	}
}

func TestFamilyConnections_PostAdultBSAID(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=PAR001")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "adult") {
		t.Errorf("expected error mentioning 'adult', got:\n%s", body)
	}
}

func TestFamilyConnections_PostYouthAlreadyHasUser(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	youthProfile, err := store.Profile.GetByBSAID(t.Context(), "YTH001")
	if err != nil {
		t.Fatalf("GetByBSAID: %v", err)
	}

	registeredUser := &user.User{PasswordHash: "hash"}
	if err := store.User.Create(t.Context(), registeredUser); err != nil {
		t.Fatalf("Create registered user: %v", err)
	}
	youthProfile.UserID = &registeredUser.ID
	if err := store.Profile.Update(t.Context(), youthProfile); err != nil {
		t.Fatalf("Update youth profile: %v", err)
	}

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=YTH001")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "already has a linked account") {
		t.Errorf("expected error about already having a linked account, got:\n%s", body)
	}
}

func TestFamilyConnections_PostSendsAdminNotification(t *testing.T) {
	handler, authService, db, emailSvc, parentProfile, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=YTH001")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	if len(emailSvc.SentNotifications) != 1 {
		t.Fatalf("expected 1 admin notification, got %d", len(emailSvc.SentNotifications))
	}

	notif := emailSvc.SentNotifications[0]
	if len(notif.To) == 0 {
		t.Fatal("expected admin notification to have recipients")
	}
	if notif.To[0] != parentProfile.Email {
		t.Errorf("expected notification to %q, got %q", parentProfile.Email, notif.To[0])
	}
	if !strings.Contains(notif.Subject, "New Family Connection") {
		t.Errorf("expected subject to mention 'New Family Connection', got %q", notif.Subject)
	}
	if !strings.Contains(notif.Body, "/admin/connections") {
		t.Errorf("expected body to contain link to /admin/connections, got: %s", notif.Body)
	}
}

func TestFamilyConnections_PostRedirectsWhenNotLoggedIn(t *testing.T) {
	handler, _, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := httptest.NewRequest("POST", "/family-connections", strings.NewReader("bsa_id=YTH001"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("AddConnection returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}
}

func TestFamilyConnections_PostEmptyBSAID(t *testing.T) {
	handler, authService, db, _, _, _ := setupFamilyConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Please enter") {
		t.Errorf("expected error 'Please enter', got:\n%s", body)
	}
}
