package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupAdminConnectionsTest(t *testing.T) (*AdminHandler, *auth.AuthService, *sql.DB, *profile.Profile) {
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

	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	handler := NewAdminHandler(store.Profile, store.ParentYouthLink, store.RBAC, authService)

	return handler, authService, db, adminProfile
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

func TestAdminConnections_GetRendersPage(t *testing.T) {
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusPending,
		RequestedAt:     time.Now(),
	}
	if err := store.ParentYouthLink.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	now := time.Now()

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}

	adminUser, err := store.User.GetByEmail(t.Context(), "admin@scout.local")
	if err != nil {
		t.Fatalf("GetByEmail admin: %v", err)
	}
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     now,
		ApprovedAt:      &now,
		ApprovedBy:      &adminUser.ID,
	}
	if err := store.ParentYouthLink.Create(t.Context(), link); err != nil {
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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

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
	handler, _, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := httptest.NewRequest("GET", "/admin/connections", nil)
	rr := httptest.NewRecorder()

	handler.ConnectionsPage(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Pending Requests") {
		t.Errorf("expected page to render, got:\n%s", body)
	}
}

func TestAdminConnections_PostApprove(t *testing.T) {
	handler, authService, db, adminProfile := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)
	adminUserID := *adminProfile.UserID

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusPending,
		RequestedAt:     time.Now(),
	}
	if err := store.ParentYouthLink.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	path := "/admin/connections/" + link.ID + "/approve"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.ApproveConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ApproveConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.ParentYouthLink.GetByID(t.Context(), link.ID)
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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusPending,
		RequestedAt:     time.Now(),
	}
	if err := store.ParentYouthLink.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	path := "/admin/connections/" + link.ID + "/reject"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RejectConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RejectConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.ParentYouthLink.GetByID(t.Context(), link.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != parentyouthlink.StatusRejected {
		t.Errorf("expected status %q, got %q", parentyouthlink.StatusRejected, updated.Status)
	}
}

func TestAdminConnections_PostRemove(t *testing.T) {
	handler, authService, db, adminProfile := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)
	adminUserID := *adminProfile.UserID

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}
	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     time.Now(),
	}
	if err := store.ParentYouthLink.Create(t.Context(), link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	path := "/admin/connections/" + link.ID + "/remove"
	req := adminConnLoggedInRequest(t, authService, "POST", path, "")
	rr := httptest.NewRecorder()

	handler.RemoveConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveConnection returned status %d, want %d", rr.Code, http.StatusOK)
	}

	updated, err := store.ParentYouthLink.GetByID(t.Context(), link.ID)
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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	req := adminConnLoggedInRequest(t, authService, "POST", "/admin/connections/nonexistent/approve", "")
	rr := httptest.NewRecorder()

	handler.ApproveConnection(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAdminConnections_GetFiltersActiveConnections(t *testing.T) {
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)
	now := time.Now()

	parent1 := &profile.Profile{
		FirstName: "Alice", LastName: "Parent", BSAID: "PAR001",
		Email: "alice@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent1); err != nil {
		t.Fatalf("Create parent1: %v", err)
	}
	youth1 := &profile.Profile{
		FirstName: "Bob", LastName: "Youth", BSAID: "YTH001",
		Email: "bob@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth1); err != nil {
		t.Fatalf("Create youth1: %v", err)
	}
	parent2 := &profile.Profile{
		FirstName: "Charlie", LastName: "Parent", BSAID: "PAR002",
		Email: "charlie@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent2); err != nil {
		t.Fatalf("Create parent2: %v", err)
	}
	youth2 := &profile.Profile{
		FirstName: "Diana", LastName: "Youth", BSAID: "YTH002",
		Email: "diana@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth2); err != nil {
		t.Fatalf("Create youth2: %v", err)
	}

	store.ParentYouthLink.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent1.ID,
		YouthProfileID:  youth1.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     now,
		ApprovedAt:      &now,
	})
	store.ParentYouthLink.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
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
	handler, authService, db, _ := setupAdminConnectionsTest(t)
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })
	store := postgres.NewStore(db)

	parent := &profile.Profile{
		FirstName: "Jane", LastName: "Parent", BSAID: "PAR001",
		Email: "jane@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	youth := &profile.Profile{
		FirstName: "Sam", LastName: "Youth", BSAID: "YTH001",
		Email: "sam@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), youth); err != nil {
		t.Fatalf("Create youth: %v", err)
	}

	store.ParentYouthLink.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusRejected,
		RequestedAt:     time.Now(),
	})
	store.ParentYouthLink.Create(t.Context(), &parentyouthlink.ParentYouthConnection{
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
