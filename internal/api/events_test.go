package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/mock"
)

func futureEvent(id string, title string, daysFromNow int) *event.Event {
	start := time.Now().AddDate(0, 0, daysFromNow)
	return &event.Event{
		ID:        id,
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func pastEvent(id string, title string, daysAgo int) *event.Event {
	start := time.Now().AddDate(0, 0, -daysAgo)
	return &event.Event{
		ID:        id,
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "meeting",
		CreatedAt: time.Now(),
	}
}

func setupEventTest(t *testing.T) (*mock.ProfileRepository, *mock.EventRepository, *mock.ParentYouthLinkRepository, *auth.AuthService, *EventHandler, *profile.Profile) {
	t.Helper()
	userRepo := mock.NewUserRepository()
	profileRepo := mock.NewProfileRepository()
	parentYouthLinkRepo := mock.NewParentYouthLinkRepository()
	rbacRepo := mock.NewRBACRepository()
	eventRepo := mock.NewEventRepository(profileRepo)

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
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := profileRepo.Create(ctx, adminProfile); err != nil {
		t.Fatalf("Create admin profile: %v", err)
	}

	handler := NewEventHandler(eventRepo, authService, rbacRepo, profileRepo, parentYouthLinkRepo, "Troop", "077")
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{"id": r.URL.Query().Get("id")}
	})

	return profileRepo, eventRepo, parentYouthLinkRepo, authService, handler, adminProfile
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
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		futureEvent("f1", "Alpha", 1),
		futureEvent("f2", "Beta", 3),
	})

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
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		pastEvent("p1", "Old Meeting", 10),
		pastEvent("p2", "Recent Campout", 2),
	})

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
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	var events []*event.Event
	for i := 0; i < 12; i++ {
		events = append(events, futureEvent(
			fmt.Sprintf("f%d", i),
			fmt.Sprintf("Event %d", i),
			i+1,
		))
	}
	eventRepo.SeedEvents(events)

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
	_, _, _, authService, handler, _ := setupEventTest(t)

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
	profileRepo, eventRepo, _, authService, handler, _ := setupEventTest(t)
	ctx := t.Context()

	attendeeProfile := &profile.Profile{
		FirstName:  "Scout",
		LastName:   "Test",
		Email:      "scout@test.com",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, attendeeProfile); err != nil {
		t.Fatalf("Create attendee profile: %v", err)
	}

	eventRepo.SeedEvents([]*event.Event{
		futureEvent("f1", "Future Campout", 2),
		pastEvent("p1", "Past Meeting", 5),
	})

	if err := eventRepo.SignUp(ctx, "f1", attendeeProfile.ID); err != nil {
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
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	req := loggedInRequest(t, authService, "GET", "/events/evt1?id=evt1")
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
	_, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	if err := eventRepo.SignUp(t.Context(), "evt1", adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/evt1?id=evt1")
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
	profileRepo, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)
	ctx := t.Context()

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	otherProfile := &profile.Profile{
		FirstName:  "Other",
		LastName:   "Scout",
		Email:      "other@scout.com",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, otherProfile); err != nil {
		t.Fatalf("Create otherProfile: %v", err)
	}

	if err := eventRepo.SignUp(ctx, "evt1", adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}
	if err := eventRepo.SignUp(ctx, "evt1", otherProfile.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/evt1?id=evt1")
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
	_, _, _, authService, handler, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/nonexistent?id=nonexistent")
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent event, got %d", rr.Code)
	}
}

func TestEventHandler_SignUp_UpdatesButtonAndAttendeeList(t *testing.T) {
	_, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)
	ctx := t.Context()

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	req := loggedInRequest(t, authService, "POST", "/events/evt1/signup?id=evt1&profile_id="+adminProfile.ID)
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

	attendees, err := eventRepo.GetAttendees(ctx, "evt1")
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee, got %d", len(attendees))
	}
}

func TestEventHandler_Withdraw_UpdatesButtonAndAttendeeList(t *testing.T) {
	_, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)
	ctx := t.Context()

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	if err := eventRepo.SignUp(ctx, "evt1", adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/evt1/withdraw?id=evt1&profile_id="+adminProfile.ID)
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

	attendees, err := eventRepo.GetAttendees(ctx, "evt1")
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees, got %d", len(attendees))
	}
}

func TestEventHandler_EventDetail_RendersEventInfo(t *testing.T) {
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{
			ID:          "evt1",
			Title:       "Campout at Lake George",
			Description: "Weekend camping trip with fun activities.",
			Location:    "Lake George",
			StartTime:   time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2026, 6, 8, 17, 0, 0, 0, time.UTC),
			CostCents:   1500,
			Type:        "campout",
			CreatedAt:   time.Now(),
		},
	})

	req := loggedInRequest(t, authService, "GET", "/events/evt1?id=evt1")
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
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{
			ID:        "past-evt",
			Title:     "Past Campout",
			Location:  "Lake George",
			StartTime: time.Now().Add(-48 * time.Hour),
			EndTime:   time.Now().Add(-46 * time.Hour),
			Type:      "campout",
			CreatedAt: time.Now(),
		},
	})

	req := loggedInRequest(t, authService, "GET", "/events/past-evt?id=past-evt")
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
	_, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{
			ID:        "past-evt",
			Title:     "Past Campout",
			Location:  "Lake",
			StartTime: time.Now().Add(-48 * time.Hour),
			EndTime:   time.Now().Add(-46 * time.Hour),
			Type:      "campout",
			CreatedAt: time.Now(),
		},
	})

	req := loggedInRequest(t, authService, "POST", "/events/past-evt/signup?id=past-evt&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SignUp returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	attendees, err := eventRepo.GetAttendees(t.Context(), "past-evt")
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees for past event, got %d", len(attendees))
	}
}

func TestEventHandler_Withdraw_PastEvent_ReturnsError(t *testing.T) {
	_, eventRepo, _, authService, handler, adminProfile := setupEventTest(t)
	ctx := t.Context()

	eventRepo.SeedEvents([]*event.Event{
		{
			ID:        "past-evt",
			Title:     "Past Campout",
			Location:  "Lake",
			StartTime: time.Now().Add(-48 * time.Hour),
			EndTime:   time.Now().Add(-46 * time.Hour),
			Type:      "campout",
			CreatedAt: time.Now(),
		},
	})

	if err := eventRepo.SignUp(ctx, "past-evt", adminProfile.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/past-evt/withdraw?id=past-evt&profile_id="+adminProfile.ID)
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Withdraw returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	attendees, err := eventRepo.GetAttendees(ctx, "past-evt")
	if err != nil {
		t.Fatalf("GetAttendees: %v", err)
	}
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee (still signed up), got %d", len(attendees))
	}
}

func TestEventHandler_EventDetail_ShowsLinkedYouthProfiles(t *testing.T) {
	profileRepo, eventRepo, parentYouthLinkRepo, authService, handler, adminProfile := setupEventTest(t)
	ctx := t.Context()

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	youthProfile := &profile.Profile{
		FirstName:  "Test",
		LastName:   "Youth",
		Email:      "test.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: adminProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusApproved,
	}
	if err := parentYouthLinkRepo.Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/evt1?id=evt1")
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("EventDetail returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	// Should show admin profile name
	if !strings.Contains(body, "Admin User") {
		t.Errorf("expected 'Admin User' in profile list, got:\n%s", body)
	}

	// Should show linked youth profile name
	if !strings.Contains(body, "Test Youth") {
		t.Errorf("expected 'Test Youth' in profile list, got:\n%s", body)
	}

	// Youth should have a Sign Up button (not signed up yet)
	if !strings.Contains(body, "Sign Up") {
		t.Errorf("expected 'Sign Up' button for youth, got:\n%s", body)
	}
}

func TestEventHandler_SignUp_MissingProfileID_ReturnsError(t *testing.T) {
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	req := loggedInRequest(t, authService, "POST", "/events/evt1/signup?id=evt1")
	rr := httptest.NewRecorder()

	handler.SignUp(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing profile_id, got %d", rr.Code)
	}
}

func TestEventHandler_Withdraw_MissingProfileID_ReturnsError(t *testing.T) {
	_, eventRepo, _, authService, handler, _ := setupEventTest(t)

	eventRepo.SeedEvents([]*event.Event{
		{ID: "evt1", Title: "Campout", Location: "Lake", StartTime: time.Now(), EndTime: time.Now().Add(2 * time.Hour), Type: "campout"},
	})

	req := loggedInRequest(t, authService, "POST", "/events/evt1/withdraw?id=evt1")
	rr := httptest.NewRecorder()

	handler.Withdraw(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing profile_id, got %d", rr.Code)
	}
}
