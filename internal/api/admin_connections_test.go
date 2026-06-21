package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/mock"
)

func setupAdminConnectionsTest(t *testing.T) (*AdminHandler, *auth.AuthService, *mock.UserRepository, *mock.ProfileRepository, *mock.ParentYouthLinkRepository, *mock.RBACRepository, *profile.Profile) {
	t.Helper()

	userRepo := mock.NewUserRepository()
	profileRepo := mock.NewProfileRepository()
	linkRepo := mock.NewParentYouthLinkRepository()
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
		BSAID:      "ADM001",
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := profileRepo.Create(ctx, adminProfile); err != nil {
		t.Fatalf("Create admin profile: %v", err)
	}

	handler := NewAdminHandler(profileRepo, linkRepo, rbacRepo, authService)

	return handler, authService, userRepo, profileRepo, linkRepo, rbacRepo, adminProfile
}

func adminConnLoggedInRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
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

func addProfile(t *testing.T, repo *mock.ProfileRepository, firstName, lastName, bsaID, email string, memberType profile.MemberType) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  firstName,
		LastName:   lastName,
		BSAID:      bsaID,
		Email:      email,
		MemberType: memberType,
		Status:     profile.StatusActive,
	}
	if err := repo.Create(t.Context(), p); err != nil {
		t.Fatalf("Create profile %s: %v", firstName, err)
	}
	return p
}

func addLink(t *testing.T, repo *mock.ParentYouthLinkRepository, parentID, youthID string, status parentyouthlink.Status) *parentyouthlink.ParentYouthConnection {
	t.Helper()
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parentID,
		YouthProfileID:  youthID,
		Status:          status,
		RequestedAt:     time.Now(),
	}
	if err := repo.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}
	return link
}

func TestAdminConnections_GetRendersPage(t *testing.T) {
	handler, authService, _, _, _, _, _ := setupAdminConnectionsTest(t)

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ConnectionsPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Pending Requests") {
		t.Errorf("expected page to contain 'Pending Requests', got:\n%s", body)
	}
	if !strings.Contains(body, "All Connections") {
		t.Errorf("expected page to contain 'All Connections', got:\n%s", body)
	}
}

func TestAdminConnections_GetShowsPendingLinks(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, _ := setupAdminConnectionsTest(t)

	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)
	addLink(t, linkRepo, parent.ID, youth.ID, parentyouthlink.StatusPending)

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Jane Parent") {
		t.Errorf("expected pending table to show parent name 'Jane Parent', got:\n%s", body)
	}
	if !strings.Contains(body, "Sam Youth") {
		t.Errorf("expected pending table to show youth name 'Sam Youth', got:\n%s", body)
	}
	if !strings.Contains(body, "YTH001") {
		t.Errorf("expected pending table to show BSA ID 'YTH001', got:\n%s", body)
	}
}

func TestAdminConnections_GetShowsActiveConnections(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, _ := setupAdminConnectionsTest(t)

	now := time.Now()
	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)
	adminID := "admin-user"
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     now,
		ApprovedAt:      &now,
		ApprovedBy:      &adminID,
	}
	if err := linkRepo.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Jane Parent") {
		t.Errorf("expected active table to show parent name 'Jane Parent', got:\n%s", body)
	}
	if !strings.Contains(body, "Sam Youth") {
		t.Errorf("expected active table to show youth name 'Sam Youth', got:\n%s", body)
	}
}

func TestAdminConnections_GetEmptyState(t *testing.T) {
	handler, authService, _, _, _, _, _ := setupAdminConnectionsTest(t)

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "No pending requests") {
		t.Errorf("expected empty state for pending, got:\n%s", body)
	}
	if !strings.Contains(body, "No connections found") {
		t.Errorf("expected empty state for connections, got:\n%s", body)
	}
}

func TestAdminConnections_GetReturnsOKWhenNotLoggedIn(t *testing.T) {
	handler, _, _, _, _, _, _ := setupAdminConnectionsTest(t)

	req := httptest.NewRequest("GET", "/admin/connections", nil)
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Pending Requests") {
		t.Errorf("expected page to render, got:\n%s", body)
	}
}

func TestAdminConnections_PostApprove(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, adminProfile := setupAdminConnectionsTest(t)
	adminUserID := *adminProfile.UserID

	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)
	link := addLink(t, linkRepo, parent.ID, youth.ID, parentyouthlink.StatusPending)

	path := "/admin/connections/" + link.ID + "/approve"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ApproveConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ApproveConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := linkRepo.GetByID(t.Context(), link.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != parentyouthlink.StatusApproved {
		t.Errorf("expected status %q, got %q", parentyouthlink.StatusApproved, updated.Status)
	}
	if updated.ApprovedBy == nil || *updated.ApprovedBy != adminUserID {
		t.Errorf("expected ApprovedBy to be %q, got %v", adminUserID, updated.ApprovedBy)
	}
	if updated.ApprovedAt == nil {
		t.Errorf("expected ApprovedAt to be set")
	}
}

func TestAdminConnections_PostReject(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, _ := setupAdminConnectionsTest(t)

	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)
	link := addLink(t, linkRepo, parent.ID, youth.ID, parentyouthlink.StatusPending)

	path := "/admin/connections/" + link.ID + "/reject"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RejectConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RejectConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := linkRepo.GetByID(t.Context(), link.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != parentyouthlink.StatusRejected {
		t.Errorf("expected status %q, got %q", parentyouthlink.StatusRejected, updated.Status)
	}
}

func TestAdminConnections_PostRemove(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, adminProfile := setupAdminConnectionsTest(t)
	adminUserID := *adminProfile.UserID

	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)
	link := addLink(t, linkRepo, parent.ID, youth.ID, parentyouthlink.StatusApproved)

	path := "/admin/connections/" + link.ID + "/remove"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RemoveConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := linkRepo.GetByID(t.Context(), link.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != parentyouthlink.StatusRevoked {
		t.Errorf("expected status %q, got %q", parentyouthlink.StatusRevoked, updated.Status)
	}
	if updated.ApprovedBy == nil || *updated.ApprovedBy != adminUserID {
		t.Errorf("expected ApprovedBy to be %q, got %v", adminUserID, updated.ApprovedBy)
	}
}

func TestAdminConnections_PostApproveInvalidID(t *testing.T) {
	handler, authService, _, _, _, _, _ := setupAdminConnectionsTest(t)

	req := adminConnLoggedInRequest(t, authService, "POST", "/admin/connections/nonexistent/approve", "")
	rr := httptest.NewRecorder()

	handler.ApproveConnection(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAdminConnections_GetFiltersActiveConnections(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, _ := setupAdminConnectionsTest(t)

	now := time.Now()
	parent1 := addProfile(t, profileRepo, "Alice", "Parent", "PAR001", "alice@test.com", profile.MemberTypeAdult)
	youth1 := addProfile(t, profileRepo, "Bob", "Youth", "YTH001", "bob@test.com", profile.MemberTypeYouth)
	parent2 := addProfile(t, profileRepo, "Charlie", "Parent", "PAR002", "charlie@test.com", profile.MemberTypeAdult)
	youth2 := addProfile(t, profileRepo, "Diana", "Youth", "YTH002", "diana@test.com", profile.MemberTypeYouth)

	linkRepo.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent1.ID,
		YouthProfileID:  youth1.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     now,
		ApprovedAt:      &now,
	})
	linkRepo.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent2.ID,
		YouthProfileID:  youth2.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     now,
		ApprovedAt:      &now,
	})

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections?search=alice", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Alice Parent") {
		t.Errorf("expected filtered results to include 'Alice Parent', got:\n%s", body)
	}
	if strings.Contains(body, "Charlie Parent") {
		t.Errorf("expected filtered results to exclude 'Charlie Parent', got:\n%s", body)
	}
}

func TestAdminConnections_GetOmitsRejectedAndRevoked(t *testing.T) {
	handler, authService, _, profileRepo, linkRepo, _, _ := setupAdminConnectionsTest(t)

	parent := addProfile(t, profileRepo, "Jane", "Parent", "PAR001", "jane@test.com", profile.MemberTypeAdult)
	youth := addProfile(t, profileRepo, "Sam", "Youth", "YTH001", "sam@test.com", profile.MemberTypeYouth)

	linkRepo.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusRejected,
		RequestedAt:     time.Now(),
	})
	linkRepo.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusRevoked,
		RequestedAt:     time.Now(),
	})

	req := adminConnLoggedInRequest(t, authService, "GET", "/admin/connections", "")
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "No pending requests") {
		t.Errorf("expected empty pending, got:\n%s", body)
	}
	if !strings.Contains(body, "Revoked") {
		t.Errorf("expected revoked connection to appear in table, got:\n%s", body)
	}
	if strings.Contains(body, "Active") {
		t.Errorf("expected no 'Active' status in table (only revoked), got:\n%s", body)
	}
}
