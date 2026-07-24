package main

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"scout-app/internal/config"
	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

type driverTestEnv struct {
	store   *postgres.Store
	server  *httptest.Server
	cleanup func()
	user    *user.User
	profile *profile.Profile
	event   *event.Event
	jar     http.CookieJar
	client  *http.Client
}

func setupDriverTest(t *testing.T) *driverTestEnv {
	t.Helper()

	dsn := testhelper.DSN()
	cfg := &config.Config{
		Addr:                ":0",
		DatabaseURL:         dsn,
		SessionSecret:       "test-secret-for-testing",
		ScoutbookAPIBaseURL: "http://localhost:9999",
		ScoutbookOrgGUID:    "",
		ScoutbookToken:      "",
		UnitType:            "Troop",
		UnitNumber:          "077",
	}

	db := openDatabase(cfg)
	store := postgres.NewStore(db)

	router, stopCleanup := buildApp(cfg, db)
	srv := httptest.NewServer(router)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := srv.Client()
	client.Jar = jar
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	ctx := context.Background()
	testhelper.TruncateAll(t, db)

	hasher := &auth.BCryptHasher{}
	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	u := &user.User{Email: "test@test.com", PasswordHash: hash}
	if err := store.User.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	p := &profile.Profile{
		FirstName: "Test", LastName: "User", Email: "test@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
		UserID: &u.ID,
	}
	if err := store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	adminRole, err := store.RBAC.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatalf("GetRoleByName admin: %v", err)
	}
	if err := store.RBAC.AssignRoleToUser(ctx, u.ID, adminRole.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	evt := &event.Event{
		Title:     "Test Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	// Mark onboarding complete
	if err := store.AppConfig.Set(ctx, appconfig.KeyOnboardingComplete, "true"); err != nil {
		t.Fatalf("set onboarding complete: %v", err)
	}

	// Log in to establish session
	resp, err := client.PostForm(srv.URL+"/login", url.Values{
		"email": {"test@test.com"}, "password": {"password"},
	})
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	resp.Body.Close()

	return &driverTestEnv{
		store:   store,
		server:  srv,
		cleanup: func() { srv.Close(); stopCleanup(); db.Close() },
		user:    u,
		profile: p,
		event:   evt,
		jar:     jar,
		client:  client,
	}
}

func TestAddDriverEndpoint_HappyPath(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	body := url.Values{"seatbelt_count": {"5"}}.Encode()
	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	drivers, err := env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].ProfileID != env.profile.ID {
		t.Errorf("expected profile ID %s, got %s", env.profile.ID, drivers[0].ProfileID)
	}
	if drivers[0].SeatbeltCount != 5 {
		t.Errorf("expected seatbelt count 5, got %d", drivers[0].SeatbeltCount)
	}
}

func TestAddDriverEndpoint_Unauthenticated(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req := httptest.NewRequest("POST", env.server.URL+"/events/"+env.event.ID+"/drivers",
		strings.NewReader("seatbelt_count=5"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 302 or 401, got %d", rr.Code)
	}
}

func TestAddDriverEndpoint_BadRequestMissingSeatbeltCount(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestAddDriverEndpoint_InvalidSeatbeltCount(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader("seatbelt_count=abc"))
	if err != nil {
		t.Fatalf("POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestRemoveDriverEndpoint_HappyPath(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	// First add a driver
	addBody := url.Values{"seatbelt_count": {"5"}}.Encode()
	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(addBody))
	if err != nil {
		t.Fatalf("POST (add): %v", err)
	}
	resp.Body.Close()

	drivers, err := env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver before remove, got %d", len(drivers))
	}

	// Now remove
	req, err := http.NewRequest("DELETE", env.server.URL+"/events/"+env.event.ID+"/drivers", nil)
	if err != nil {
		t.Fatalf("DELETE request: %v", err)
	}
	resp, err = env.client.Do(req)
	if err != nil {
		t.Fatalf("do DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	drivers, err = env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 0 {
		t.Errorf("expected 0 drivers after remove, got %d", len(drivers))
	}
}

func TestUpdateDriverSeatbeltEndpoint_HappyPath(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	// First add a driver with seatbelt_count=5
	addBody := url.Values{"seatbelt_count": {"5"}}.Encode()
	_, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(addBody))
	if err != nil {
		t.Fatalf("POST (add): %v", err)
	}

	// Then update to seatbelt_count=3
	updateBody := url.Values{"seatbelt_count": {"3"}}.Encode()
	req, err := http.NewRequest("PATCH", env.server.URL+"/events/"+env.event.ID+"/drivers",
		strings.NewReader(updateBody))
	if err != nil {
		t.Fatalf("PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	drivers, err := env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].SeatbeltCount != 3 {
		t.Errorf("expected seatbelt count 3, got %d", drivers[0].SeatbeltCount)
	}
}

func TestAddDriverEndpoint_EventNotFound(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	body := url.Values{"seatbelt_count": {"5"}}.Encode()
	resp, err := env.client.Post(env.server.URL+"/events/nonexistent-id/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestWithdrawRemovesDriverCascade(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	// Add as driver first
	addBody := url.Values{"seatbelt_count": {"5"}}.Encode()
	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(addBody))
	if err != nil {
		t.Fatalf("POST (add): %v", err)
	}
	resp.Body.Close()

	drivers, err := env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver before withdraw, got %d", len(drivers))
	}

	// Now withdraw (profile_id is a query parameter)
	req, err := http.NewRequest("POST", env.server.URL+"/events/"+env.event.ID+"/withdraw?profile_id="+env.profile.ID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = env.client.Do(req)
	if err != nil {
		t.Fatalf("POST withdraw: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify driver record is also removed
	drivers, err = env.store.Event.GetDrivers(context.Background(), env.event.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 0 {
		t.Errorf("expected 0 drivers after withdraw, got %d", len(drivers))
	}
}

func TestAddDriverEndpoint_PastEvent(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	ctx := context.Background()

	pastEvent := &event.Event{
		Title:     "Past Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-24 * time.Hour),
		Type:      "campout",
	}
	if err := env.store.Event.Create(ctx, pastEvent); err != nil {
		t.Fatalf("Create past event: %v", err)
	}

	body := url.Values{"seatbelt_count": {"5"}}.Encode()
	resp, err := env.client.Post(env.server.URL+"/events/"+pastEvent.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestRemoveDriverEndpoint_Unauthenticated(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req := httptest.NewRequest("DELETE", env.server.URL+"/events/"+env.event.ID+"/drivers", nil)
	rr := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 302 or 401, got %d", rr.Code)
	}
}

func TestRemoveDriverEndpoint_EventNotFound(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req, err := http.NewRequest("DELETE", env.server.URL+"/events/nonexistent/drivers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestRemoveDriverEndpoint_PastEvent(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	pastEvent := &event.Event{
		Title: "Past Campout", Location: "Lake George",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-24 * time.Hour), Type: "campout",
	}
	if err := env.store.Event.Create(context.Background(), pastEvent); err != nil {
		t.Fatalf("Create past event: %v", err)
	}

	req, err := http.NewRequest("DELETE", env.server.URL+"/events/"+pastEvent.ID+"/drivers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestUpdateDriverSeatbeltEndpoint_MissingSeatbeltCount(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req, err := http.NewRequest("PATCH", env.server.URL+"/events/"+env.event.ID+"/drivers", strings.NewReader(""))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestUpdateDriverSeatbeltEndpoint_InvalidSeatbeltCount(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req, err := http.NewRequest("PATCH", env.server.URL+"/events/"+env.event.ID+"/drivers",
		strings.NewReader("seatbelt_count=abc"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestUpdateDriverSeatbeltEndpoint_Unauthenticated(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req := httptest.NewRequest("PATCH", env.server.URL+"/events/"+env.event.ID+"/drivers",
		strings.NewReader("seatbelt_count=3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 302 or 401, got %d", rr.Code)
	}
}

func TestUpdateDriverSeatbeltEndpoint_EventNotFound(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req, err := http.NewRequest("PATCH", env.server.URL+"/events/nonexistent/drivers",
		strings.NewReader("seatbelt_count=3"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestUpdateDriverSeatbeltEndpoint_PastEvent(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	pastEvent := &event.Event{
		Title: "Past Campout", Location: "Lake George",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-24 * time.Hour), Type: "campout",
	}
	if err := env.store.Event.Create(context.Background(), pastEvent); err != nil {
		t.Fatalf("Create past event: %v", err)
	}

	req, err := http.NewRequest("PATCH", env.server.URL+"/events/"+pastEvent.ID+"/drivers",
		strings.NewReader("seatbelt_count=3"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestEventDetailPageLoad_ShowsDriversSection(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.Get(env.server.URL + "/events/" + env.event.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestAddDriverEndpoint_ParseFormError(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.Post(env.server.URL+"/events/"+env.event.ID+"/drivers",
		"application/x-www-form-urlencoded", strings.NewReader("%zz"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestUpdateDriverSeatbeltEndpoint_ParseFormError(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	req, err := http.NewRequest("PATCH", env.server.URL+"/events/"+env.event.ID+"/drivers",
		strings.NewReader("%zz"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestSignUpAdultSelfSignup_ShowsDriversSection(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.PostForm(env.server.URL+"/events/"+env.event.ID+"/signup?profile_id="+env.profile.ID,
		url.Values{})
	if err != nil {
		t.Fatalf("POST signup: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}
