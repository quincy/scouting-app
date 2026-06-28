package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
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
	if err := auth.SeedRoles(ctx, store.RBAC); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}

	_, adminProfile := seedAdminUser(t, store, hasher, ctx)

	handler := NewEventHandler(store.Event, authService, store.RBAC, store.Profile, store.ParentYouthLink, "Troop", "077")
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{"id": r.URL.Query().Get("id")}
	})

	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	return handler, authService, store, adminProfile
}

func loggedInRequest(t *testing.T, authService *auth.AuthService, method, path string) *http.Request {
	t.Helper()

	authHandler := NewAuthHandler(authService)
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
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
	if !strings.Contains(body, "hx-post=\"/admin/markdown-preview\"") {
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
