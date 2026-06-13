package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/mock"
)

func setupFamilyConnectionsTest(t *testing.T) (*FamilyConnectionsHandler, *auth.AuthService, *mock.ProfileRepository, *mock.ParentYouthLinkRepository, *profile.Profile, *profile.Profile) {
	t.Helper()

	userRepo := mock.NewUserRepository()
	profileRepo := mock.NewProfileRepository()
	parentYouthLinkRepo := mock.NewParentYouthLinkRepository()
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
	parentProfile := &profile.Profile{
		FirstName:  "Parent",
		LastName:   "User",
		BSAID:      "PAR001",
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := profileRepo.Create(ctx, parentProfile); err != nil {
		t.Fatalf("Create parent profile: %v", err)
	}

	youthProfile := &profile.Profile{
		FirstName:  "Alex",
		LastName:   "Youth",
		BSAID:      "YTH001",
		Email:      "alex.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	handler := NewFamilyConnectionsHandler(profileRepo, parentYouthLinkRepo, authService, rbacRepo)

	return handler, authService, profileRepo, parentYouthLinkRepo, parentProfile, youthProfile
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
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

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
	handler, _, _, _, _, _ := setupFamilyConnectionsTest(t)

	req := httptest.NewRequest("GET", "/family-connections", nil)
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("FamilyConnectionsPage returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}
}

func TestFamilyConnections_GetShowsEmptyState(t *testing.T) {
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

	req := familyConnLoggedInRequest(t, authService, "GET", "/family-connections", "")
	rr := httptest.NewRecorder()

	handler.FamilyConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "No family connections yet") {
		t.Errorf("expected empty state message, got:\n%s", body)
	}
}

func TestFamilyConnections_GetShowsFormForAdult(t *testing.T) {
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

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
	_, _, profileRepo, linkRepo, _, youthProfile := setupFamilyConnectionsTest(t)

	ctx := t.Context()

	youthProfile.UserID = &youthProfile.ID
	if err := profileRepo.Update(ctx, youthProfile); err != nil {
		t.Fatalf("Update youth profile: %v", err)
	}

	rbacRepo2 := mock.NewRBACRepository()
	if err := rbacRepo2.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}

	userRepo2 := mock.NewUserRepository()
	youthUser := &user.User{Email: "alex.youth@scout.local", PasswordHash: "password"}
	if err := userRepo2.Create(ctx, youthUser); err != nil {
		t.Fatalf("Create youth user: %v", err)
	}

	hasher2 := &auth.MockHasher{}
	store2 := auth.NewCookieStore("test-secret-key")
	authService2 := auth.NewAuthService(userRepo2, rbacRepo2, hasher2, store2)

	handler2 := NewFamilyConnectionsHandler(profileRepo, linkRepo, authService2, rbacRepo2)

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
	handler, authService, _, linkRepo, parentProfile, youthProfile := setupFamilyConnectionsTest(t)

	ctx := t.Context()
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parentProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
	}
	if err := linkRepo.Create(ctx, link); err != nil {
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
	handler, authService, _, linkRepo, parentProfile, _ := setupFamilyConnectionsTest(t)

	req := familyConnLoggedInRequest(t, authService, "POST", "/family-connections", "bsa_id=YTH001")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	links, err := linkRepo.ListByParent(t.Context(), parentProfile.ID)
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
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

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
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

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
	handler, authService, profileRepo, _, _, _ := setupFamilyConnectionsTest(t)

	ctx := t.Context()
	youthProfile, err := profileRepo.GetByBSAID(ctx, "YTH001")
	if err != nil {
		t.Fatalf("GetByBSAID: %v", err)
	}

	registeredUserID := "registered-user-id"
	youthProfile.UserID = &registeredUserID
	if err := profileRepo.Update(ctx, youthProfile); err != nil {
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

func TestFamilyConnections_PostRedirectsWhenNotLoggedIn(t *testing.T) {
	handler, _, _, _, _, _ := setupFamilyConnectionsTest(t)

	req := httptest.NewRequest("POST", "/family-connections", strings.NewReader("bsa_id=YTH001"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.AddConnection(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("AddConnection returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}
}

func TestFamilyConnections_PostEmptyBSAID(t *testing.T) {
	handler, authService, _, _, _, _ := setupFamilyConnectionsTest(t)

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
