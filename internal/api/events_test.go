package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func futureEvent(title string, daysFromNow int) *event.Event {
	start := time.Now().AddDate(0, 0, daysFromNow)
	return &event.Event{
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func pastEvent(title string, daysAgo int) *event.Event {
	start := time.Now().AddDate(0, 0, -daysAgo)
	return &event.Event{
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func setupEventTest(t *testing.T) (*EventHandler, *auth.AuthService, *postgres.Store, *profile.Profile) {
	t.Helper()

	db := testhelper.StartDB()
	store := postgres.NewStore(db)

	hasher := &auth.MockHasher{}
	cookieStore := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(store.User, store.Profile, store.RBAC, hasher, cookieStore)

	ctx := t.Context()
	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	appCfg := appconfig.NewInMemoryRepository()
	appCfg.Set(ctx, appconfig.KeyUnitType, "Troop")
	appCfg.Set(ctx, appconfig.KeyUnitNumber, "077")

	handler := NewEventHandler(store.Event, authService, store.RBAC, store.Profile, store.ParentYouthLink, appCfg)
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{"id": r.URL.Query().Get("id")}
	})

	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	return handler, authService, store, adminProfile
}

func loggedInRequest(t *testing.T, authService *auth.AuthService, method, path string) *http.Request {
	t.Helper()
	return loggedInAs(t, authService, method, path, "admin@scout.local")
}

func loggedInAs(t *testing.T, authService *auth.AuthService, method, path, email string) *http.Request {
	t.Helper()

	authHandler := NewAuthHandler(authService)
	body := url.Values{"email": {email}, "password": {"password"}}
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(body.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)

	req := httptest.NewRequest(method, path, nil)
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestEventHandler_ListUpcomingPartial(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	for _, e := range []*event.Event{
		futureEvent("Alpha", 1),
		futureEvent("Beta", 3),
	} {
		if err := store.Event.Create(ctx, e); err != nil {
			t.Fatalf("Create event: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/events/upcoming?offset=0", nil)
	rr := httptest.NewRecorder()

	handler.ListUpcoming(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListUpcoming returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Alpha") {
		t.Errorf("expected partial to contain 'Alpha', got:\n%s", body)
	}
	if !strings.Contains(body, "Beta") {
		t.Errorf("expected partial to contain 'Beta', got:\n%s", body)
	}

	if !strings.Contains(body, "upcoming-count") {
		t.Errorf("expected partial to contain OOB counter, got:\n%s", body)
	}
	if !strings.Contains(body, "Showing 2 of 2") {
		t.Errorf("expected partial to say 'Showing 2 of 2', got:\n%s", body)
	}

	_ = authService
}

func TestEventHandler_ListPastPartial(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	for _, e := range []*event.Event{
		pastEvent("Old Meeting", 10),
		pastEvent("Recent Campout", 2),
	} {
		if err := store.Event.Create(ctx, e); err != nil {
			t.Fatalf("Create event: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/events/past?offset=0", nil)
	rr := httptest.NewRecorder()

	handler.ListPast(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListPast returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Old Meeting") {
		t.Errorf("expected partial to contain 'Old Meeting', got:\n%s", body)
	}
	if !strings.Contains(body, "Recent Campout") {
		t.Errorf("expected partial to contain 'Recent Campout', got:\n%s", body)
	}
	if !strings.Contains(body, "Showing 2 of 2") {
		t.Errorf("expected partial to say 'Showing 2 of 2', got:\n%s", body)
	}

	_ = authService
}

func TestEventHandler_ListUpcoming_Pagination(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	for i := 0; i < 12; i++ {
		e := futureEvent(
			fmt.Sprintf("Event %d", i),
			i+1,
		)
		if err := store.Event.Create(ctx, e); err != nil {
			t.Fatalf("Create event: %v", err)
		}
	}

	t.Run("first page returns 10 events with ShowMore", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/upcoming?offset=0", nil)
		rr := httptest.NewRecorder()
		handler.ListUpcoming(rr, req)

		body := rr.Body.String()
		if !strings.Contains(body, "Event 0") {
			t.Errorf("expected first event, got:\n%s", body)
		}
		if !strings.Contains(body, "Event 9") {
			t.Errorf("expected tenth event 'Event 9', got:\n%s", body)
		}
		if strings.Contains(body, "Event 10") {
			t.Error("did not expect Event 10 on first page")
		}

		if !strings.Contains(body, "Showing 10 of 12") {
			t.Errorf("expected 'Showing 10 of 12', got:\n%s", body)
		}

		if !strings.Contains(body, "show-more-upcoming") {
			t.Errorf("expected show-more button to be present, got:\n%s", body)
		}
	})

	t.Run("second page returns remaining 2 events and hides button", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/upcoming?offset=10", nil)
		rr := httptest.NewRecorder()
		handler.ListUpcoming(rr, req)

		body := rr.Body.String()
		if !strings.Contains(body, "Event 10") {
			t.Errorf("expected Event 10 on second page, got:\n%s", body)
		}
		if !strings.Contains(body, "Event 11") {
			t.Errorf("expected Event 11 on second page, got:\n%s", body)
		}

		if !strings.Contains(body, "Showing 12 of 12") {
			t.Errorf("expected 'Showing 12 of 12', got:\n%s", body)
		}
	})

	_ = authService
}

func TestEventHandler_ListEvents_Empty(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()

	handler.ListEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Showing 0 of 0") {
		t.Errorf("expected empty counts, got body:\n%s", body)
	}

	_ = authService
}

func TestEventHandler_ListEvents(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	attendeeProfile := &profile.Profile{
		FirstName:  "Scout",
		LastName:   "Test",
		Email:      "scout@test.com",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, attendeeProfile); err != nil {
		t.Fatalf("Create attendee profile: %v", err)
	}

	future := futureEvent("Future Campout", 2)
	past := pastEvent("Past Meeting", 5)
	for _, e := range []*event.Event{future, past} {
		if err := store.Event.Create(ctx, e); err != nil {
			t.Fatalf("Create event: %v", err)
		}
	}

	if err := store.Event.SignUp(ctx, future.ID, attendeeProfile.ID); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()

	handler.ListEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	ct := rr.Header().Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}

	if !strings.Contains(body, "Future Campout") {
		t.Errorf("expected page to contain 'Future Campout', got:\n%s", body)
	}
	if !strings.Contains(body, "Past Meeting") {
		t.Errorf("expected page to contain 'Past Meeting', got:\n%s", body)
	}
	if !strings.Contains(body, "campout") {
		t.Errorf("expected page to contain event type 'campout'")
	}
	if !strings.Contains(body, "1 attendee") {
		t.Errorf("expected page to contain '1 attendee'")
	}
	if !strings.Contains(body, "Upcoming Events") {
		t.Errorf("expected page to contain 'Upcoming Events' section")
	}
	if !strings.Contains(body, "Past Events") {
		t.Errorf("expected page to contain 'Past Events' section")
	}

	_ = authService
}

func TestEventHandler_EventDetail_ShowsSignUpButtonWhenNotAttending(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Sign Up") {
		t.Errorf("expected 'Sign Up' button when not attending, got:\n%s", body)
	}
	if strings.Contains(body, "Withdraw") {
		t.Error("expected no 'Withdraw' button when not attending")
	}
}

func TestEventHandler_EventDetail_ShowsWithdrawButtonWhenAttending(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Withdraw") {
		t.Errorf("expected 'Withdraw' button when attending, got:\n%s", body)
	}
	if strings.Contains(body, "Sign Up") {
		t.Error("expected no 'Sign Up' button when attending")
	}
}

func TestEventHandler_EventDetail_ShowsProfileNameInAttendeeList(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "Scout",
		Email:      "other@scout.com",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create otherProfile: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, otherProfile.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()

	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected 'Admin User' in attendee list, got:\n%s", body)
	}
	if !strings.Contains(body, "Other Scout") {
		t.Errorf("expected 'Other Scout' in attendee list, got:\n%s", body)
	}
}

func TestEventHandler_EventDetail_ShowsDriverBadgeInAttendeeList(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	// Create a second adult profile that is NOT a driver
	otherAdult := &profile.Profile{
		FirstName: "Other", LastName: "Adult", Email: "other@scout.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherAdult); err != nil {
		t.Fatalf("Create otherAdult: %v", err)
	}

	// Sign up both adults
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, otherAdult.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	// Add admin as a driver
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()

	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected 'Admin User' in attendee list, got:\n%s", body)
	}
	if !strings.Contains(body, "Other Adult") {
		t.Errorf("expected 'Other Adult' in attendee list, got:\n%s", body)
	}
	if !strings.Contains(body, "resp-on") {
		t.Errorf("expected driver badge (resp-on) in attendee list, got:\n%s", body)
	}
	if !strings.Contains(body, "5 seat") {
		t.Errorf("expected seatbelt count in driver badge, got:\n%s", body)
	}
}

func TestEventHandler_EventDetail_DriverTableShowsDriverNames(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()

	if !strings.Contains(body, "driver-table") {
		t.Errorf("expected driver table in drivers section, got:\n%s", body)
	}
}

func TestEventHandler_SignUp_ShowsDriverModalForAdult(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "driver-modal-overlay") {
		t.Errorf("expected driver modal overlay in signup response for adult, got:\n%s", body)
	}
	if !strings.Contains(body, "Sign up as driver") {
		t.Errorf("expected 'Sign up as driver' in signup response for adult, got:\n%s", body)
	}
}

func TestEventHandler_SignUp_DriverModalSubmitDoesNotRemoveOverlayPrematurely(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SignUp returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "driver-modal-overlay") {
		t.Fatalf("expected driver modal overlay in signup response for adult, got:\n%s", body)
	}

	if strings.Contains(body, "setTimeout") {
		t.Errorf("driver modal submit button must not remove the overlay before the response arrives (removing the form mid-flight breaks htmx OOB swaps under latency), got setTimeout in:\n%s", body)
	}
}

func TestEventHandler_EventDetail_ShowsSeatbeltBadge(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	body := rr.Body.String()

	if !strings.Contains(body, "id=\"seatbelt-badge\"") {
		t.Errorf("expected seatbelt-badge element in event detail, got:\n%s", body)
	}
	if !strings.Contains(body, "5 / 1 seatbelts") {
		t.Errorf("expected '5 / 1 seatbelts' in seatbelt badge, got:\n%s", body)
	}
}

func TestEventHandler_EventDetail_NonExistentEventReturns404(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/nonexistent?id=nonexistent")
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent event, got %d", rr.Code)
	}
}

func TestEventHandler_SignUp_UpdatesButtonAndAttendeeList(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Withdraw") {
		t.Errorf("expected 'Withdraw' button after signup, got:\n%s", body)
	}
	if strings.Contains(body, "Sign Up") {
		t.Error("expected no 'Sign Up' button after signup")
	}

	if !strings.Contains(body, "attendee-count") {
		t.Errorf("expected attendee-count OOB element, got:\n%s", body)
	}

	attendees, err := store.Event.GetAttendees(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee, got %d", len(attendees))
	}
}

func TestEventHandler_Withdraw_UpdatesButtonAndAttendeeList(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/withdraw?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Withdraw returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Sign Up") {
		t.Errorf("expected 'Sign Up' button after withdraw, got:\n%s", body)
	}
	if strings.Contains(body, "Withdraw") {
		t.Error("expected no 'Withdraw' button after withdraw")
	}

	if !strings.Contains(body, "attendee-count") {
		t.Errorf("expected attendee-count OOB element, got:\n%s", body)
	}

	attendees, err := store.Event.GetAttendees(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees, got %d", len(attendees))
	}
}

func TestEventHandler_Withdraw_UpdatesDriversSection(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/withdraw?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Withdraw returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "No drivers signed up yet") {
		t.Errorf("expected drivers section to show empty state after withdraw, got:\n%s", body)
	}
}

func TestEventHandler_EventDetail_RendersEventInfo(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:       "Campout at Lake George",
		Description: "Weekend camping trip with fun activities.",
		Location:    "Lake George",
		StartTime:   time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents:   1500,
		Type:        "campout",
		CreatedAt:   time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDetail returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Campout at Lake George") {
		t.Errorf("expected title in response, got:\n%s", body)
	}
	if !strings.Contains(body, "Weekend camping trip with fun activities.") {
		t.Errorf("expected description in response, got:\n%s", body)
	}
	if !strings.Contains(body, "Lake George") {
		t.Errorf("expected location in response, got:\n%s", body)
	}
	if !strings.Contains(body, "campout") {
		t.Errorf("expected event type 'campout' in response, got:\n%s", body)
	}
	if !strings.Contains(body, "$15.00") {
		t.Errorf("expected formatted cost '$15.00' in response, got:\n%s", body)
	}
	if !strings.Contains(body, "Back to events") {
		t.Errorf("expected back link in response, got:\n%s", body)
	}

	_ = authService
}

func TestEventHandler_EventDetail_PastEvent_ShowsEndedMessage(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Past Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-46 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDetail returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "This event has ended") {
		t.Errorf("expected 'This event has ended' message for past event, got:\n%s", body)
	}
	if strings.Contains(body, "Sign Up") {
		t.Error("expected no 'Sign Up' button for past event")
	}
	if strings.Contains(body, "Withdraw") {
		t.Error("expected no 'Withdraw' button for past event")
	}
}

func TestEventHandler_SignUp_PastEvent_ReturnsError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Past Campout",
		Location:  "Lake",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-46 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	attendees, err := store.Event.GetAttendees(t.Context(), evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees for past event, got %d", len(attendees))
	}
}

func TestEventHandler_Withdraw_PastEvent_ReturnsError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Past Campout",
		Location:  "Lake",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-46 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/withdraw?id="+evt.ID+"&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Withdraw returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	attendees, err := store.Event.GetAttendees(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee (still signed up), got %d", len(attendees))
	}
}

func TestEventHandler_EventDetail_ShowsLinkedYouthProfiles(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	youthProfile := &profile.Profile{
		FirstName:  "Test",
		LastName:   "Youth",
		Email:      "test.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: adminProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
	}
	if err := store.ParentYouthLink.Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDetail returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected 'Admin User' in profile list, got:\n%s", body)
	}

	if !strings.Contains(body, "Test Youth") {
		t.Errorf("expected 'Test Youth' in profile list, got:\n%s", body)
	}

	if !strings.Contains(body, "Sign Up") {
		t.Errorf("expected 'Sign Up' button for youth, got:\n%s", body)
	}
}

func TestEventHandler_SignUp_MissingProfileID_ReturnsError(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing profile_id, got %d", rr.Code)
	}
}

func TestEventHandler_Withdraw_MissingProfileID_ReturnsError(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/withdraw?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing profile_id, got %d", rr.Code)
	}
}

func TestEventHandler_EventCreateForm_Renders(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/create")
	rr := httptest.NewRecorder()

	handler.EventCreateForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventCreateForm returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Create Event") {
		t.Errorf("expected form to contain 'Create Event', got:\n%s", body)
	}
	if !strings.Contains(body, "event-form") {
		t.Errorf("expected form to have 'event-form' class, got:\n%s", body)
	}
	if !strings.Contains(body, "split-editor") {
		t.Errorf("expected form to have split editor, got:\n%s", body)
	}
	if !strings.Contains(body, "hx-post=\"/events/markdown-preview\"") {
		t.Errorf("expected textarea to have htmx markdown preview trigger, got:\n%s", body)
	}
}

func loggedInPostRequest(t *testing.T, authService *auth.AuthService, path string, form url.Values) *http.Request {
	t.Helper()

	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)

	body := strings.NewReader(form.Encode())
	req := httptest.NewRequest("POST", path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestEventHandler_EventCreate_Success(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)

	form := url.Values{
		"title":       {"Test Event"},
		"description": {"Test description"},
		"location":    {"Test Location"},
		"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"25.00"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventCreate returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/events/") {
		t.Errorf("expected redirect to /events/{id}, got Location: %s", location)
	}
	if !strings.Contains(location, "created=1") {
		t.Errorf("expected redirect to include ?created=1, got Location: %s", location)
	}

	eventID := strings.TrimPrefix(strings.Split(location, "?")[0], "/events/")
	created, err := store.Event.GetByID(t.Context(), eventID)
	if err != nil {
		t.Fatalf("expected created event to exist, got error: %v", err)
	}
	if created.Title != "Test Event" {
		t.Errorf("expected event title 'Test Event', got %q", created.Title)
	}
	if created.Description != "Test description" {
		t.Errorf("expected event description 'Test description', got %q", created.Description)
	}
	if created.Location != "Test Location" {
		t.Errorf("expected event location 'Test Location', got %q", created.Location)
	}
	if created.CostCents != 2500 {
		t.Errorf("expected cost 2500 cents, got %d", created.CostCents)
	}
	if created.Type != "campout" {
		t.Errorf("expected type 'campout', got %q", created.Type)
	}
}

func TestEventHandler_EventCreateForm_RendersToggles(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/create")
	rr := httptest.NewRecorder()

	handler.EventCreateForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventCreateForm returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `name="cooking_enabled"`) {
		t.Errorf("expected cooking toggle checkbox, got:\n%s", body)
	}
	if !strings.Contains(body, `name="tenting_enabled"`) {
		t.Errorf("expected tenting toggle checkbox, got:\n%s", body)
	}
}

func TestEventHandler_EventCreate_PersistsToggles(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)

	form := url.Values{
		"title":           {"Test Event"},
		"description":     {"Test description"},
		"location":        {"Test Location"},
		"start_time":      {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":        {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"cost":            {"25.00"},
		"type":            {"campout"},
		"cooking_enabled": {"on"},
		"tenting_enabled": {"on"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventCreate returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	eventID := strings.TrimPrefix(strings.Split(location, "?")[0], "/events/")
	created, err := store.Event.GetByID(t.Context(), eventID)
	if err != nil {
		t.Fatalf("expected created event to exist, got error: %v", err)
	}
	if !created.CookingEnabled {
		t.Error("expected CookingEnabled to be true")
	}
	if !created.TentingEnabled {
		t.Error("expected TentingEnabled to be true")
	}
}

func TestEventHandler_EventCreate_UncheckedTogglesDefaultFalse(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)

	form := url.Values{
		"title":       {"Test Event"},
		"description": {"Test description"},
		"location":    {"Test Location"},
		"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"25.00"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventCreate returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	eventID := strings.TrimPrefix(strings.Split(location, "?")[0], "/events/")
	created, err := store.Event.GetByID(t.Context(), eventID)
	if err != nil {
		t.Fatalf("expected created event to exist, got error: %v", err)
	}
	if created.CookingEnabled {
		t.Error("expected CookingEnabled to default to false")
	}
	if created.TentingEnabled {
		t.Error("expected TentingEnabled to default to false")
	}
}

func TestEventHandler_EventCreate_ValidationError(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	form := url.Values{
		"title":       {""},
		"description": {""},
		"location":    {""},
		"start_time":  {""},
		"end_time":    {""},
		"cost":        {""},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("EventCreate returned status %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	bodyStr := rr.Body.String()
	if !strings.Contains(bodyStr, "Title is required") {
		t.Errorf("expected 'Title is required' error, got:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Location is required") {
		t.Errorf("expected 'Location is required' error, got:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Start time is required") {
		t.Errorf("expected 'Start time is required' error, got:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "End time is required") {
		t.Errorf("expected 'End time is required' error, got:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Cost is required") {
		t.Errorf("expected 'Cost is required' error, got:\n%s", bodyStr)
	}
}

func TestEventHandler_EventCreate_PreservesFormValuesOnError(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	form := url.Values{
		"title":       {"Campout at Yosemite"},
		"description": {"A fun weekend trip"},
		"location":    {""},
		"start_time":  {time.Now().Add(72 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(96 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"15.00"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("EventCreate returned status %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Campout at Yosemite") {
		t.Errorf("expected title to be preserved, got:\n%s", body)
	}
	if !strings.Contains(body, "A fun weekend trip") {
		t.Errorf("expected description to be preserved, got:\n%s", body)
	}
	if !strings.Contains(body, "Location is required") {
		t.Errorf("expected 'Location is required' error, got:\n%s", body)
	}
	if !strings.Contains(body, "15.00") {
		t.Errorf("expected cost '15.00' to be preserved, got:\n%s", body)
	}
}

func TestEventHandler_EventCreate_EndTimeBeforeStart(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	form := url.Values{
		"title":       {"Test Event"},
		"description": {""},
		"location":    {"Somewhere"},
		"start_time":  {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"10"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("EventCreate returned status %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	if !strings.Contains(rr.Body.String(), "End time must be after start time") {
		t.Errorf("expected end time validation error, got:\n%s", rr.Body.String())
	}
}

func TestEventHandler_EventCreate_InvalidCost(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	form := url.Values{
		"title":       {"Test Event"},
		"description": {""},
		"location":    {"Somewhere"},
		"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"abc"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()

	handler.EventCreate(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("EventCreate returned status %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	if !strings.Contains(rr.Body.String(), "Invalid cost value") {
		t.Errorf("expected 'Invalid cost value' error, got:\n%s", rr.Body.String())
	}
}

func setAuthCookie(t *testing.T, authService *auth.AuthService, req *http.Request) {
	t.Helper()
	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
}

func TestEventHandler_EventDeleteConfirm_Renders(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout at Lake George", Location: "Lake George", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := httptest.NewRequest("GET", "/events/"+evt.ID+"/delete?id="+evt.ID, nil)
	setAuthCookie(t, authService, req)
	rr := httptest.NewRecorder()

	handler.EventDeleteConfirm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDeleteConfirm returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Campout at Lake George") {
		t.Errorf("expected event title in confirm modal, got:\n%s", body)
	}
	if !strings.Contains(body, "Are you sure") {
		t.Errorf("expected confirmation text, got:\n%s", body)
	}
	if !strings.Contains(body, "hx-delete") {
		t.Errorf("expected hx-delete attribute in form, got:\n%s", body)
	}
}

func TestEventHandler_EventDelete_Success(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{Title: "Campout at Lake George", Location: "Lake George", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/events/"+evt.ID+"/delete?id="+evt.ID, nil)
	setAuthCookie(t, authService, req)
	rr := httptest.NewRecorder()

	handler.EventDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDelete returned status %d, want %d", rr.Code, http.StatusOK)
	}

	if rr.Header().Get("HX-Redirect") != "/events?deleted=1" {
		t.Errorf("expected HX-Redirect header, got %q", rr.Header().Get("HX-Redirect"))
	}

	_, err := store.Event.GetByID(t.Context(), evt.ID)
	if err == nil {
		t.Error("expected event to be deleted")
	}
}

func TestEventHandler_EventDelete_NotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := httptest.NewRequest("DELETE", "/events/evt1/delete?id=evt1", nil)
	setAuthCookie(t, authService, req)
	rr := httptest.NewRecorder()

	handler.EventDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("EventDelete returned status %d, want %d", rr.Code, http.StatusNotFound)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Failed to delete event") {
		t.Errorf("expected 'Failed to delete event' error, got:\n%s", body)
	}
}

func TestEventHandler_EventEditForm_Renders(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:       "Campout at Lake George",
		Description: "Weekend camping trip.",
		Location:    "Lake George",
		StartTime:   time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents:   1500,
		Type:        "campout",
		CreatedAt:   time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	// Read back from DB to get time as handler will see it
	savedEvt, err := store.Event.GetByID(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/edit?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventEditForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventEditForm returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Edit Event") {
		t.Errorf("expected 'Edit Event' title, got:\n%s", body)
	}
	if !strings.Contains(body, "Save Changes") {
		t.Errorf("expected 'Save Changes' submit label, got:\n%s", body)
	}
	if !strings.Contains(body, "Campout at Lake George") {
		t.Errorf("expected pre-filled title, got:\n%s", body)
	}
	if !strings.Contains(body, "Weekend camping trip") {
		t.Errorf("expected pre-filled description, got:\n%s", body)
	}

	expectedStart := savedEvt.StartTime.Format("2006-01-02T15:04")
	if !strings.Contains(body, expectedStart) {
		t.Errorf("expected pre-filled start time %q, got:\n%s", expectedStart, body)
	}
}

func TestEventHandler_EventEdit_Success(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:       "Original Title",
		Description: "Original description",
		Location:    "Original Location",
		StartTime:   time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents:   1000,
		Type:        "campout",
		CreatedAt:   time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	form := url.Values{
		"title":       {"Updated Title"},
		"description": {"Updated description"},
		"location":    {"Updated Location"},
		"start_time":  {time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC).Format("2006-01-02T15:04")},
		"end_time":    {time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC).Format("2006-01-02T15:04")},
		"cost":        {"25.00"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/"+evt.ID+"/edit?id="+evt.ID, form)
	rr := httptest.NewRecorder()

	handler.EventEdit(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventEdit returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/events/"+evt.ID) {
		t.Errorf("expected redirect to /events/%s, got Location: %s", evt.ID, location)
	}
	if !strings.Contains(location, "updated=1") {
		t.Errorf("expected redirect to include ?updated=1, got Location: %s", location)
	}

	updated, err := store.Event.GetByID(ctx, evt.ID)
	if err != nil {
		t.Fatalf("expected updated event to exist, got error: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", updated.Title)
	}
	if updated.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", updated.Description)
	}
	if updated.Location != "Updated Location" {
		t.Errorf("expected location 'Updated Location', got %q", updated.Location)
	}
	if updated.CostCents != 2500 {
		t.Errorf("expected cost 2500 cents, got %d", updated.CostCents)
	}
	if updated.Type != "campout" {
		t.Errorf("expected type 'campout', got %q", updated.Type)
	}
}

func TestEventHandler_EventEdit_PersistsToggles(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:     "Original Title",
		Location:  "Original Location",
		StartTime: time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents: 1000,
		Type:      "campout",
		CreatedAt: time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	form := url.Values{
		"title":           {"Updated Title"},
		"description":     {"Updated description"},
		"location":        {"Updated Location"},
		"start_time":      {time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC).Format("2006-01-02T15:04")},
		"end_time":        {time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC).Format("2006-01-02T15:04")},
		"cost":            {"25.00"},
		"type":            {"campout"},
		"cooking_enabled": {"on"},
		"tenting_enabled": {"on"},
	}

	req := loggedInPostRequest(t, authService, "/events/"+evt.ID+"/edit?id="+evt.ID, form)
	rr := httptest.NewRecorder()

	handler.EventEdit(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventEdit returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	updated, err := store.Event.GetByID(ctx, evt.ID)
	if err != nil {
		t.Fatalf("expected updated event to exist, got error: %v", err)
	}
	if !updated.CookingEnabled {
		t.Error("expected CookingEnabled to be true after edit")
	}
	if !updated.TentingEnabled {
		t.Error("expected TentingEnabled to be true after edit")
	}
}

func TestEventHandler_EventEditForm_RendersCheckedToggles(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:          "Campout",
		Location:       "Lake George",
		StartTime:      time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:        time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents:      1500,
		Type:           "campout",
		CookingEnabled: true,
		TentingEnabled: true,
		CreatedAt:      time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/edit?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventEditForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventEditForm returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `name="cooking_enabled" checked`) {
		t.Errorf("expected cooking toggle checked, got:\n%s", body)
	}
	if !strings.Contains(body, `name="tenting_enabled" checked`) {
		t.Errorf("expected tenting toggle checked, got:\n%s", body)
	}
}

func TestEventHandler_EventEdit_NotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/nonexistent/edit?id=nonexistent")
	rr := httptest.NewRecorder()

	handler.EventEditForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("EventEditForm returned status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestEventHandler_EventEdit_ValidationError(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := &event.Event{
		Title:       "Original Title",
		Description: "Original description",
		Location:    "Original Location",
		StartTime:   time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
		CostCents:   1000,
		Type:        "campout",
		CreatedAt:   time.Now(),
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	form := url.Values{
		"title":       {""},
		"description": {""},
		"location":    {""},
		"start_time":  {""},
		"end_time":    {""},
		"cost":        {""},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/"+evt.ID+"/edit?id="+evt.ID, form)
	rr := httptest.NewRecorder()

	handler.EventEdit(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("EventEdit returned status %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Title is required") {
		t.Errorf("expected 'Title is required' error, got:\n%s", body)
	}
	if !strings.Contains(body, "Location is required") {
		t.Errorf("expected 'Location is required' error, got:\n%s", body)
	}
	if !strings.Contains(body, "Start time is required") {
		t.Errorf("expected 'Start time is required' error, got:\n%s", body)
	}
	if !strings.Contains(body, "End time is required") {
		t.Errorf("expected 'End time is required' error, got:\n%s", body)
	}
	if !strings.Contains(body, "Cost is required") {
		t.Errorf("expected 'Cost is required' error, got:\n%s", body)
	}
}

func TestEventHandler_SignUp_ParentCanSignUpYouth(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName:  "Young",
		LastName:   "Scout",
		Email:      "young.scout@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: adminProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     time.Now(),
		CreatedAt:       time.Now(),
	}
	if err := store.ParentYouthLink.Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+youthProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestEventHandler_SignUp_ForbiddenForUnrelatedProfile(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "Person",
		Email:      "other@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	evt := &event.Event{Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+otherProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func loggedInBodyRequest(t *testing.T, authService *auth.AuthService, method, path, body string) *http.Request {
	t.Helper()

	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.Login(loginRR, loginReq)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestEventHandler_AddDriver_HappyPath(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusOK)
	}

	drivers, err := store.Event.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].SeatbeltCount != 5 {
		t.Errorf("expected seatbelt count 5, got %d", drivers[0].SeatbeltCount)
	}
}

func TestEventHandler_AddDriver_UpdatesAttendeeListBadge(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "resp-on") {
		t.Errorf("expected driver badge (resp-on) in attendee list after AddDriver, got:\n%s", body)
	}
}

func TestEventHandler_RemoveDriver_HappyPath(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/drivers?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusOK)
	}

	drivers, err := store.Event.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 0 {
		t.Errorf("expected 0 drivers, got %d", len(drivers))
	}
}

func TestEventHandler_UpdateDriverSeatbelt_HappyPath(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=3")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusOK)
	}

	drivers, err := store.Event.GetDrivers(ctx, evt.ID)
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

func TestEventHandler_AddDriver_PastEvent(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := pastEvent("Past Campout", 1)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_RemoveDriver_PastEvent(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := pastEvent("Past Campout", 1)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/drivers?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_MissingSeatbeltCount(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_InvalidSeatbeltCount(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=abc")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_PastEvent(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()

	evt := pastEvent("Past Campout", 1)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=3")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_AddDriver_MissingSeatbeltCount(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_AddDriver_InvalidSeatbeltCount(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=abc")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_AddDriver_Unauthenticated(t *testing.T) {
	handler, _, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := httptest.NewRequest("POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, strings.NewReader("seatbelt_count=5"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("AddDriver returned %d, want 302 or 401", rr.Code)
	}
}

func TestEventHandler_AddDriver_EventNotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInBodyRequest(t, authService, "POST", "/events/nonexistent/drivers?id=nonexistent", "seatbelt_count=5")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestEventHandler_AddDriver_ParseFormError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "%zz")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_AddDriver_GetByUserIDError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	// Login as admin, repoint profile to a different user so GetByUserID fails
	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	hasher := &auth.BCryptHasher{}
	h, err := hasher.Hash("other")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	otherUser := &user.User{Email: "other@test.com", PasswordHash: h}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}
	adminProfile.UserID = &otherUser.ID
	if err := store.Profile.Update(ctx, adminProfile); err != nil {
		t.Fatalf("Update profile: %v", err)
	}
	rr := httptest.NewRecorder()
	handler.AddDriver(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestEventHandler_RemoveDriver_Unauthenticated(t *testing.T) {
	handler, _, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/events/"+evt.ID+"/drivers?id="+evt.ID, nil)
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("RemoveDriver returned %d, want 302 or 401", rr.Code)
	}
}

func TestEventHandler_RemoveDriver_EventNotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "DELETE", "/events/nonexistent/drivers?id=nonexistent")
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestEventHandler_RemoveDriver_GetByUserIDError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/drivers?id="+evt.ID)
	hasher := &auth.BCryptHasher{}
	h, err := hasher.Hash("remove")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	otherUser := &user.User{Email: "other-remove@test.com", PasswordHash: h}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}
	adminProfile.UserID = &otherUser.ID
	if err := store.Profile.Update(ctx, adminProfile); err != nil {
		t.Fatalf("Update profile: %v", err)
	}
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_Unauthenticated(t *testing.T) {
	handler, _, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, strings.NewReader("seatbelt_count=3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusFound && rr.Code != http.StatusUnauthorized {
		t.Errorf("UpdateDriverSeatbelt returned %d, want 302 or 401", rr.Code)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_EventNotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/nonexistent/drivers?id=nonexistent", "seatbelt_count=3")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_ParseFormError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "%zz")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_GetByUserIDError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "PATCH", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=3")
	hasher := &auth.BCryptHasher{}
	h, err := hasher.Hash("update")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	otherUser := &user.User{Email: "other-update@test.com", PasswordHash: h}
	if err := store.User.Create(ctx, otherUser); err != nil {
		t.Fatalf("Create other user: %v", err)
	}
	adminProfile.UserID = &otherUser.ID
	if err := store.Profile.Update(ctx, adminProfile); err != nil {
		t.Fatalf("Update profile: %v", err)
	}
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestEventHandler_AddDriver_EmptyEventID(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInBodyRequest(t, authService, "POST", "/events//drivers", "seatbelt_count=5")
	rr := httptest.NewRecorder()

	handler.AddDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("AddDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_RemoveDriver_EmptyEventID(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "DELETE", "/events//drivers")
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventHandler_UpdateDriverSeatbelt_EmptyEventID(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInBodyRequest(t, authService, "PATCH", "/events//drivers", "seatbelt_count=3")
	rr := httptest.NewRecorder()

	handler.UpdateDriverSeatbelt(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateDriverSeatbelt returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

type failingEventRepo struct {
	event.Repository
	failOnRemoveDriver bool
	failOnGetByID      bool
	failOnGetAttendees bool
}

func (r *failingEventRepo) RemoveDriver(ctx context.Context, eventID string, profileID string) error {
	if r.failOnRemoveDriver {
		return errors.New("injected error")
	}
	return r.Repository.RemoveDriver(ctx, eventID, profileID)
}

func (r *failingEventRepo) GetByID(ctx context.Context, eventID string) (*event.Event, error) {
	if r.failOnGetByID {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetByID(ctx, eventID)
}

func (r *failingEventRepo) GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error) {
	if r.failOnGetAttendees {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetAttendees(ctx, eventID)
}

func TestEventHandler_EventCreate_AutoAssignsCoordinator(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)

	form := url.Values{
		"title":       {"Coordinator Test"},
		"description": {"Testing auto-assign coordinator"},
		"location":    {"Test Location"},
		"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"end_time":    {time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")},
		"cost":        {"0.00"},
		"type":        {"campout"},
	}

	req := loggedInPostRequest(t, authService, "/events/create", form)
	rr := httptest.NewRecorder()
	handler.EventCreate(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("EventCreate returned %d, want %d", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	eventID := strings.TrimPrefix(strings.Split(location, "?")[0], "/events/")

	holder, err := store.Event.GetResponsibilityHolder(t.Context(), eventID, event.ResponsibilityCoordinator)
	if err != nil {
		t.Fatalf("GetResponsibilityHolder: %v", err)
	}
	if holder == nil {
		t.Fatal("expected coordinator to be auto-assigned, got nil holder")
	}
	if holder.ProfileID != adminProfile.ID {
		t.Errorf("expected coordinator to be %s, got %s", adminProfile.ID, holder.ProfileID)
	}
}

func TestEventHandler_SignUp_AutoAssignsSPL_WhenYouthHasPosition(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName:  "Young",
		LastName:   "SPL",
		Email:      "young.spl@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
		Positions:  "Senior Patrol Leader",
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: adminProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     time.Now(),
		CreatedAt:       time.Now(),
	}
	if err := store.ParentYouthLink.Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	evt := &event.Event{
		Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout",
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+youthProfile.ID)
	rr := httptest.NewRecorder()
	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SignUp returned %d, want %d", rr.Code, http.StatusOK)
	}

	holder, err := store.Event.GetResponsibilityHolder(ctx, evt.ID, event.ResponsibilitySPL)
	if err != nil {
		t.Fatalf("GetResponsibilityHolder: %v", err)
	}
	if holder == nil {
		t.Fatal("expected SPL to be auto-assigned, got nil holder")
	}
	if holder.ProfileID != youthProfile.ID {
		t.Errorf("expected SPL to be %s, got %s", youthProfile.ID, holder.ProfileID)
	}
}

func TestEventHandler_SignUp_DoesNotAutoAssignSPL_WhenYouthHasNoPosition(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName:  "Young",
		LastName:   "Scout",
		Email:      "young.scout2@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
		Positions:  "",
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: adminProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
		RequestedAt:     time.Now(),
		CreatedAt:       time.Now(),
	}
	if err := store.ParentYouthLink.Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	evt := &event.Event{
		Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout",
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/signup?id="+evt.ID+"&profile_id="+youthProfile.ID)
	rr := httptest.NewRecorder()
	handler.SignUp(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SignUp returned %d, want %d", rr.Code, http.StatusOK)
	}

	holder, err := store.Event.GetResponsibilityHolder(ctx, evt.ID, event.ResponsibilitySPL)
	if err != nil {
		t.Fatalf("GetResponsibilityHolder: %v", err)
	}
	if holder != nil {
		t.Errorf("expected no SPL assignment for youth without SPL position, got holder %s", holder.ProfileID)
	}
}

func TestEventHandler_RemoveDriver_RepoError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adminProfile.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	handler.repo = &failingEventRepo{Repository: store.Event, failOnRemoveDriver: true}

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/drivers?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RemoveDriver(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("RemoveDriver returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestEnrichVMsWithResponsibilities(t *testing.T) {
	vms := []attendeeViewModel{
		{ProfileID: "p1", ProfileName: "Alice"},
		{ProfileID: "p2", ProfileName: "Bob"},
	}
	responsibilities := []event.ResponsibilityAssignment{
		{ProfileID: "p1", Responsibility: event.ResponsibilitySPL},
		{ProfileID: "p1", Responsibility: event.ResponsibilityCoordinator},
		{ProfileID: "p2", Responsibility: event.ResponsibilityMedicalOfficer},
	}
	enrichVMsWithResponsibilities(vms, responsibilities)

	if !vms[0].IsSPL {
		t.Errorf("Alice should be SPL")
	}
	if !vms[0].IsCoordinator {
		t.Errorf("Alice should be Coordinator")
	}
	if vms[0].IsMedicalOfficer {
		t.Errorf("Alice should not be Medical Officer")
	}
	if vms[1].IsSPL {
		t.Errorf("Bob should not be SPL")
	}
	if vms[1].IsCoordinator {
		t.Errorf("Bob should not be Coordinator")
	}
	if !vms[1].IsMedicalOfficer {
		t.Errorf("Bob should be Medical Officer")
	}
}

func TestEnrichVMsWithResponsibilities_NoResponsibilities(t *testing.T) {
	vms := []attendeeViewModel{
		{ProfileID: "p1", ProfileName: "Alice"},
	}
	enrichVMsWithResponsibilities(vms, nil)

	if vms[0].IsSPL {
		t.Errorf("no responsibilities should not set IsSPL")
	}
}

func TestEnrichVMsWithResponsibilities_EmptyVMs(t *testing.T) {
	enrichVMsWithResponsibilities(nil, []event.ResponsibilityAssignment{
		{ProfileID: "p1", Responsibility: event.ResponsibilitySPL},
	})
}

func setupToggleMux() func() {
	orig := muxVars
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{
			"id":             r.URL.Query().Get("id"),
			"profile_id":     r.URL.Query().Get("profile_id"),
			"responsibility": r.URL.Query().Get("responsibility"),
		}
	})
	return func() { muxVars = orig }
}

func toggleURL(eventID, profileID, responsibility string) string {
	return fmt.Sprintf("/events/%s/responsibility/%s/%s?id=%s&profile_id=%s&responsibility=%s",
		eventID, profileID, responsibility, eventID, profileID, responsibility)
}

func replaceURL(eventID, profileID, responsibility, currentHolderID string) string {
	return fmt.Sprintf("/events/%s/replace-responsibility/%s/%s?id=%s&profile_id=%s&responsibility=%s&current_holder_id=%s",
		eventID, profileID, responsibility, eventID, profileID, responsibility, currentHolderID)
}

func TestToggleResponsibility_AssignSPL(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName: "Young", LastName: "Scout", Email: "youth@scout.com",
		MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, youthProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, youthProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resp-on") {
		t.Errorf("expected resp-on badge in response, got:\n%s", rr.Body.String())
	}
}

func TestToggleResponsibility_RemoveSPL(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName: "Young", LastName: "Scout", Email: "youth2@scout.com",
		MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, youthProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := store.Event.AssignResponsibility(ctx, evt.ID, youthProfile.ID, event.ResponsibilitySPL); err != nil {
		t.Fatalf("Assign SPL: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, youthProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resp-off") {
		t.Errorf("expected resp-off badge after removing SPL, got:\n%s", rr.Body.String())
	}
}

func TestToggleResponsibility_PastEvent(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := &event.Event{
		Title: "Past Event", Location: "Lake",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-24 * time.Hour),
		Type:      "campout",
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestToggleResponsibility_Unauthenticated(t *testing.T) {
	handler, _, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := httptest.NewRequest("POST", "/events/123/responsibility/p1/spl?id=123&profile_id=p1&responsibility=spl", nil)
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestToggleResponsibility_NotSignedUp(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestToggleResponsibility_DriverReturnsBadRequest(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "driver"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestToggleResponsibility_Coordinator(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "coordinator"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resp-on") {
		t.Errorf("expected resp-on badge for Coordinator, got:\n%s", rr.Body.String())
	}
}

func TestToggleResponsibility_MedicalOfficer(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "medical_officer"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resp-on") {
		t.Errorf("expected resp-on badge for Medical Officer, got:\n%s", rr.Body.String())
	}
}

func TestToggleResponsibility_SingletonConflict(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "User",
		Email:      "other@scout.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, otherProfile.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}
	if err := store.Event.AssignResponsibility(ctx, evt.ID, adminProfile.ID, event.ResponsibilitySPL); err != nil {
		t.Fatalf("Assign SPL to admin: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, otherProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ToggleResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "toast-error") {
		t.Errorf("unexpected toast-error, expected confirmation modal. Body:\n%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Change SPL?") {
		t.Errorf("expected confirmation modal with 'Change SPL?' in response. Body:\n%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), adminProfile.DisplayName()) {
		t.Errorf("expected confirmation modal to name the current holder %q. Body:\n%s", adminProfile.DisplayName(), rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Other User") {
		t.Errorf("expected confirmation modal to name the requested profile. Body:\n%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "replace-responsibility") {
		t.Errorf("expected confirmation modal to have replace-responsibility endpoint. Body:\n%s", rr.Body.String())
	}
}

func TestToggleResponsibility_EventNotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := loggedInRequest(t, authService, "POST", "/events/nonexistent/responsibility/p1/spl?id=nonexistent&profile_id=p1&responsibility=spl")
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestToggleResponsibility_EmptyPathParams(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := loggedInRequest(t, authService, "POST", "/events//responsibility//spl?id=&profile_id=&responsibility=spl")
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestToggleResponsibility_GetByIDError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	handler.repo = &failingEventRepo{Repository: store.Event, failOnGetByID: true}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestToggleResponsibility_GetAttendeesError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	handler.repo = &failingEventRepo{Repository: store.Event, failOnGetAttendees: true}

	req := loggedInRequest(t, authService, "POST", toggleURL(evt.ID, adminProfile.ID, "spl"))
	rr := httptest.NewRecorder()
	handler.ToggleResponsibility(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ToggleResponsibility returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestReplaceResponsibility_Success(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	youthProfile := &profile.Profile{
		FirstName: "Young", LastName: "Scout", Email: "youth@scout.com",
		MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, youthProfile.ID); err != nil {
		t.Fatalf("SignUp youth: %v", err)
	}
	if err := store.Event.AssignResponsibility(ctx, evt.ID, adminProfile.ID, event.ResponsibilitySPL); err != nil {
		t.Fatalf("Assign SPL to admin: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", replaceURL(evt.ID, youthProfile.ID, "spl", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ReplaceResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resp-on") {
		t.Errorf("expected resp-on badge in response, got:\n%s", rr.Body.String())
	}

	resp, err := store.Event.GetResponsibilities(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetResponsibilities: %v", err)
	}
	foundAssignments := 0
	for _, ra := range resp {
		if ra.Responsibility == event.ResponsibilitySPL {
			foundAssignments++
			if ra.ProfileID != youthProfile.ID {
				t.Errorf("SPL assigned to %s, want %s", ra.ProfileID, youthProfile.ID)
			}
		}
	}
	if foundAssignments != 1 {
		t.Errorf("expected 1 SPL assignment, got %d", foundAssignments)
	}
}

func TestReplaceResponsibility_PastEvent(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	otherProfile := &profile.Profile{
		FirstName: "Other", LastName: "User", Email: "other@scout.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	evt := pastEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, otherProfile.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", replaceURL(evt.ID, otherProfile.ID, "spl", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReplaceResponsibility_Unauthenticated(t *testing.T) {
	handler, _, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := httptest.NewRequest("POST", replaceURL("e1", "p1", "spl", "p2"), nil)
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestReplaceResponsibility_NotSignedUp(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	otherProfile := &profile.Profile{
		FirstName: "Other", LastName: "User", Email: "other@scout.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", replaceURL(evt.ID, otherProfile.ID, "spl", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReplaceResponsibility returned %d, want %d. Body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestReplaceResponsibility_EmptyParams(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := loggedInRequest(t, authService, "POST", "/events//replace-responsibility//spl?id=&profile_id=&responsibility=spl&current_holder_id=")
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReplaceResponsibility_DriverReturnsBadRequest(t *testing.T) {
	handler, authService, _, adminProfile := setupEventTest(t)
	defer setupToggleMux()()

	req := loggedInRequest(t, authService, "POST", replaceURL("e1", "p1", "driver", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReplaceResponsibility_EventNotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupToggleMux()()

	req := loggedInRequest(t, authService, "POST", "/events/nonexistent/replace-responsibility/p1/spl?id=nonexistent&profile_id=p1&responsibility=spl&current_holder_id=p2")
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestReplaceResponsibility_GetByIDError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	handler.repo = &failingEventRepo{Repository: store.Event, failOnGetByID: true}

	req := loggedInRequest(t, authService, "POST", replaceURL(evt.ID, adminProfile.ID, "spl", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestReplaceResponsibility_GetAttendeesError(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	handler.repo = &failingEventRepo{Repository: store.Event, failOnGetAttendees: true}

	req := loggedInRequest(t, authService, "POST", replaceURL(evt.ID, adminProfile.ID, "spl", adminProfile.ID))
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestReplaceResponsibility_Forbidden(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupToggleMux()()
	ctx := t.Context()

	hasher := &auth.MockHasher{}
	hash, _ := hasher.Hash("password")
	regularUser := &user.User{
		Email:        "regular@scout.com",
		PasswordHash: hash,
	}
	if err := store.User.Create(ctx, regularUser); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	nonAdminProfile := &profile.Profile{
		FirstName: "Regular", LastName: "User", Email: "regular@scout.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
		UserID: &regularUser.ID,
	}
	if err := store.Profile.Create(ctx, nonAdminProfile); err != nil {
		t.Fatalf("Create non-admin profile: %v", err)
	}

	otherProfile := &profile.Profile{
		FirstName: "Other", LastName: "Adult", Email: "other@scout.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := store.Profile.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create other profile: %v", err)
	}

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, otherProfile.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	// non-admin tries to manage otherProfile (not themselves) - should be forbidden
	req := loggedInAs(t, authService, "POST", replaceURL(evt.ID, otherProfile.ID, "spl", otherProfile.ID), "regular@scout.com")
	rr := httptest.NewRecorder()
	handler.ReplaceResponsibility(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("ReplaceResponsibility returned %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestResponsibilityLabel(t *testing.T) {
	tests := []struct {
		input    event.Responsibility
		expected string
	}{
		{event.ResponsibilitySPL, "SPL"},
		{event.ResponsibilityCoordinator, "Coordinator"},
		{event.ResponsibilityMedicalOfficer, "Medical Officer"},
		{event.Responsibility("unknown"), "unknown"},
	}
	for _, tc := range tests {
		got := responsibilityLabel(tc.input)
		if got != tc.expected {
			t.Errorf("responsibilityLabel(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestAttendeeSection_HasOobSwap(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	rr := httptest.NewRecorder()
	handler.AddDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("AddDriver returned %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `id="attendee-section" hx-swap-oob="true"`) {
		t.Errorf("expected attendee-section to have hx-swap-oob=\"true\", got:\n%s", body)
	}
}

func TestDriverBadge_Pluralization_Multiple(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=5")
	rr := httptest.NewRecorder()
	handler.AddDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("AddDriver returned %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "5 seats") {
		t.Errorf("expected '5 seats' in driver badge, got:\n%s", body)
	}
}

func TestDriverBadge_Pluralization_Singular(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()

	evt := futureEvent("Campout", 7)
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/drivers?id="+evt.ID, "seatbelt_count=1")
	rr := httptest.NewRecorder()
	handler.AddDriver(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("AddDriver returned %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "1 seat") {
		t.Errorf("expected '1 seat' in driver badge, got:\n%s", body)
	}
	if strings.Contains(body, "1 seats") {
		t.Errorf("expected singular '1 seat', got '1 seats':\n%s", body)
	}
}
