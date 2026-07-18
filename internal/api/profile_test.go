package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func setupProfileTest(t *testing.T) (*ProfileHandler, *auth.AuthService, *postgres.Store, *profile.Profile) {
	t.Helper()

	db := testhelper.StartDB()
	store := postgres.NewStore(db)

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()
	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	handler := NewProfileHandler(store.Profile, store.Event, authService, store.RBAC, store.ParentYouthLink)
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{"id": r.URL.Query().Get("id")}
	})

	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	return handler, authService, store, adminProfile
}

func profileLoggedInRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
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

func TestProfilePage_GetReturnsProfilePage(t *testing.T) {
	handler, authService, _, adminProfile := setupProfileTest(t)

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+adminProfile.ID+"?id="+adminProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, adminProfile.DisplayName()) {
		t.Errorf("expected page to contain profile name %q, got:\n%s", adminProfile.DisplayName(), body)
	}
}

func TestProfilePage_NotFound(t *testing.T) {
	handler, authService, _, _ := setupProfileTest(t)

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/nonexistent?id=nonexistent", "")
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProfilePage_Unauthenticated(t *testing.T) {
	handler, _, store, _ := setupProfileTest(t)

	someProfile := &profile.Profile{
		FirstName:  "Some",
		LastName:   "Body",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), someProfile); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	req := httptest.NewRequest("GET", "/profiles/"+someProfile.ID+"?id="+someProfile.ID, nil)
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestProfilePage_ShowsNotRegisteredForUnlinkedProfile(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)

	unlinkedProfile := &profile.Profile{
		FirstName:  "Unlinked",
		LastName:   "User",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), unlinkedProfile); err != nil {
		t.Fatalf("Create unlinked profile: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+unlinkedProfile.ID+"?id="+unlinkedProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Not registered") {
		t.Errorf("expected 'Not registered' for unlinked profile, got:\n%s", body)
	}
}

func TestProfilePage_ShowsAdminSectionForAdminViewingRegisteredAdult(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)

	ctx := t.Context()

	regUser := &user.User{
		Email:        "registered@scout.local",
		PasswordHash: "hash",
	}
	if err := store.User.Create(ctx, regUser); err != nil {
		t.Fatalf("Create registered user: %v", err)
	}

	regProfile := &profile.Profile{
		FirstName:  "Registered",
		LastName:   "Adult",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &regUser.ID,
	}
	if err := store.Profile.Create(ctx, regProfile); err != nil {
		t.Fatalf("Create registered profile: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+regProfile.ID+"?id="+regProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Grant Admin") {
		t.Errorf("expected 'Grant Admin' button in admin section, got:\n%s", body)
	}
}

func TestProfilePage_HidesAdminSectionForYouth(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)

	ctx := t.Context()

	youthUser := &user.User{
		Email:        "youth@scout.local",
		PasswordHash: "hash",
	}
	if err := store.User.Create(ctx, youthUser); err != nil {
		t.Fatalf("Create youth user: %v", err)
	}

	youthProfile := &profile.Profile{
		FirstName:  "Young",
		LastName:   "Person",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
		UserID:     &youthUser.ID,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+youthProfile.ID+"?id="+youthProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfilePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfilePage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if strings.Contains(body, "Admin Actions") {
		t.Errorf("expected no admin section for youth profile, but found it")
	}
}

func TestProfileUpcomingEvents_RendersPartial(t *testing.T) {
	handler, authService, store, adminProfile := setupProfileTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Upcoming Campout",
		Location:  "Camp Ground",
		StartTime: time.Now().AddDate(0, 0, 5),
		EndTime:   time.Now().AddDate(0, 0, 5).Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+adminProfile.ID+"/events/upcoming?id="+adminProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfileUpcomingEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfileUpcomingEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Upcoming Campout") {
		t.Errorf("expected event title in partial, got:\n%s", body)
	}
}

func TestProfilePastEvents_RendersPartial(t *testing.T) {
	handler, authService, store, adminProfile := setupProfileTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Past Campout",
		Location:  "Camp Ground",
		StartTime: time.Now().AddDate(0, 0, -5),
		EndTime:   time.Now().AddDate(0, 0, -5).Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "GET", "/profiles/"+adminProfile.ID+"/events/past?id="+adminProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.ProfilePastEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ProfilePastEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Past Campout") {
		t.Errorf("expected event title in partial, got:\n%s", body)
	}
}

func TestProfileEvents_BadRequest(t *testing.T) {
	handler, authService, _, _ := setupProfileTest(t)

	req := profileLoggedInRequest(t, authService, "GET", "/profiles//events/upcoming?id=", "")
	rr := httptest.NewRecorder()

	handler.ProfileUpcomingEvents(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProfileUpcomingEvents returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGrantAdmin_GrantsAndRendersSection(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)
	ctx := t.Context()

	targetUser := &user.User{
		Email:        "target@scout.local",
		PasswordHash: "hash",
	}
	if err := store.User.Create(ctx, targetUser); err != nil {
		t.Fatalf("Create target user: %v", err)
	}

	targetProfile := &profile.Profile{
		FirstName:  "Target",
		LastName:   "User",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &targetUser.ID,
	}
	if err := store.Profile.Create(ctx, targetProfile); err != nil {
		t.Fatalf("Create target profile: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "POST", "/profiles/"+targetProfile.ID+"/grant-admin?id="+targetProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Revoke Admin") {
		t.Errorf("expected 'Revoke Admin' button after grant, got:\n%s", body)
	}

	roles, err := store.RBAC.GetUserRoles(ctx, targetUser.ID)
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
		t.Error("expected target user to have admin role after grant")
	}
}

func TestGrantAdmin_SelfGrantFails(t *testing.T) {
	handler, authService, _, adminProfile := setupProfileTest(t)

	req := profileLoggedInRequest(t, authService, "POST", "/profiles/"+adminProfile.ID+"/grant-admin?id="+adminProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRemoveAdmin_RemovesAndRendersSection(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)
	ctx := t.Context()

	targetUser := &user.User{
		Email:        "target2@scout.local",
		PasswordHash: "hash",
	}
	if err := store.User.Create(ctx, targetUser); err != nil {
		t.Fatalf("Create target user: %v", err)
	}

	targetProfile := &profile.Profile{
		FirstName:  "Target2",
		LastName:   "User",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &targetUser.ID,
	}
	if err := store.Profile.Create(ctx, targetProfile); err != nil {
		t.Fatalf("Create target profile: %v", err)
	}

	adminRole, err := store.RBAC.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName: %v", err)
	}
	if err := store.RBAC.AssignRoleToUser(ctx, targetUser.ID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "POST", "/profiles/"+targetProfile.ID+"/remove-admin?id="+targetProfile.ID, "")
	rr := httptest.NewRecorder()

	handler.RemoveAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveAdmin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Grant Admin") {
		t.Errorf("expected 'Grant Admin' button after removal, got:\n%s", body)
	}

	roles, err := store.RBAC.GetUserRoles(ctx, targetUser.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	for _, role := range roles {
		if role.Name == "admin" {
			t.Error("expected target user to no longer have admin role after removal")
		}
	}
}

func TestToggleAdmin_ProfileNotFound(t *testing.T) {
	handler, authService, _, _ := setupProfileTest(t)

	req := profileLoggedInRequest(t, authService, "POST", "/profiles/nonexistent/grant-admin?id=nonexistent", "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestToggleAdmin_UnregisteredProfile(t *testing.T) {
	handler, authService, store, _ := setupProfileTest(t)

	unlinked := &profile.Profile{
		FirstName:  "NoUser",
		LastName:   "Profile",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), unlinked); err != nil {
		t.Fatalf("Create unlinked profile: %v", err)
	}

	req := profileLoggedInRequest(t, authService, "POST", "/profiles/"+unlinked.ID+"/grant-admin?id="+unlinked.ID, "")
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestToggleAdmin_Unauthenticated(t *testing.T) {
	handler, _, store, _ := setupProfileTest(t)

	someProfile := &profile.Profile{
		FirstName:  "Some",
		LastName:   "Body",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(t.Context(), someProfile); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	req := httptest.NewRequest("POST", "/profiles/"+someProfile.ID+"/grant-admin?id="+someProfile.ID, nil)
	rr := httptest.NewRecorder()

	handler.GrantAdmin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GrantAdmin returned status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
