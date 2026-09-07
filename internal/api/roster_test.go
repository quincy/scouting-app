package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/postgres"
)

func rosterEvent(t *testing.T, store *postgres.Store) *event.Event {
	t.Helper()
	evt := &event.Event{
		Title:          "Roster Campout",
		Location:       "Lake",
		StartTime:      time.Now().Add(24 * time.Hour),
		EndTime:        time.Now().Add(48 * time.Hour),
		Type:           "campout",
		CookingEnabled: true,
		TentingEnabled: true,
		DriversEnabled: true,
	}
	if err := store.Event.Create(t.Context(), evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	return evt
}

func TestRosterPage_ForbiddenForNonManager(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	evt := rosterEvent(t, store)
	createParentUser(t, store)

	req := loggedInAs(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID, "parent@scout.local")
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestEventDetail_ShowsRosterLinkForAdmin(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	evt := rosterEvent(t, store)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "/events/"+evt.ID+"/roster") {
		t.Errorf("expected roster link on event detail page:\n%s", body)
	}
}

func TestRosterPage_NotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events/missing/roster?id=missing")
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func createAdultAttendee(t *testing.T, store *postgres.Store, name string, birthdate time.Time) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  name,
		LastName:   "Parent",
		Email:      name + ".parent@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		Birthdate:  birthdate,
	}
	if err := store.Profile.Create(t.Context(), p); err != nil {
		t.Fatalf("Create adult profile: %v", err)
	}
	return p
}

func createYouthAttendee(t *testing.T, store *postgres.Store, name string, birthdate time.Time) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  name,
		LastName:   "Scout",
		Email:      name + ".scout@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
		Birthdate:  birthdate,
	}
	if err := store.Profile.Create(t.Context(), p); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}
	return p
}

func TestRosterPage_RendersAttendanceWithResponsibilitiesAndAges(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	ctx := t.Context()
	evt := rosterEvent(t, store)

	youth := createYouthAttendee(t, store, "Avery", time.Now().AddDate(-13, 0, 0))
	adult := createAdultAttendee(t, store, "Casey", time.Now().AddDate(-45, 0, 0))

	if err := store.Event.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatalf("SignUp youth: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatalf("SignUp adult: %v", err)
	}
	if err := store.Event.SignUp(ctx, evt.ID, adminProfile.ID); err != nil {
		t.Fatalf("SignUp admin: %v", err)
	}

	if err := store.Event.AssignResponsibility(ctx, evt.ID, youth.ID, event.ResponsibilitySPL); err != nil {
		t.Fatalf("AssignResponsibility SPL: %v", err)
	}
	if err := store.Event.AssignResponsibility(ctx, evt.ID, adminProfile.ID, event.ResponsibilityCoordinator); err != nil {
		t.Fatalf("AssignResponsibility Coordinator: %v", err)
	}
	if err := store.Event.AssignResponsibility(ctx, evt.ID, adult.ID, event.ResponsibilityMedicalOfficer); err != nil {
		t.Fatalf("AssignResponsibility MedicalOfficer: %v", err)
	}
	if err := store.Event.AddDriver(ctx, evt.ID, adult.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()

	for _, want := range []string{
		"Attendance (Roster)",
		"Youth / Scouts",
		"Adults",
		"Avery Scout",
		"Casey Parent",
		"Admin User",
		"SPL",
		"Coord",
		"Medical Officer",
		"5 seats",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in roster body:\n%s", want, body)
		}
	}
}

func TestRosterPage_RendersCookingPatrols(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()
	evt := rosterEvent(t, store)

	cook := createYouthAttendee(t, store, "Eli", time.Now().AddDate(-14, 0, 0))
	member := createYouthAttendee(t, store, "Noah", time.Now().AddDate(-13, 0, 0))
	signUpAttendee(t, store, evt.ID, cook.ID)
	signUpAttendee(t, store, evt.ID, member.ID)

	patrol, err := store.Event.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("CreateCookingPatrol: %v", err)
	}
	if err := store.Event.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, cook.ID); err != nil {
		t.Fatalf("AssignCookingPatrolMember: %v", err)
	}
	if err := store.Event.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, member.ID); err != nil {
		t.Fatalf("AssignCookingPatrolMember: %v", err)
	}
	if err := store.Event.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, cook.ID); err != nil {
		t.Fatalf("SetCookingPatrolCook: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()

	for _, want := range []string{
		"Cooking Patrols",
		"Cooking 1",
		"Eli Scout",
		"Noah Scout",
		"Cook",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in roster body:\n%s", want, body)
		}
	}
}

func TestRosterPage_RendersTentingAssignmentsWithUnassigned(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()
	evt := rosterEvent(t, store)

	tented := createYouthAttendee(t, store, "Mia", time.Now().AddDate(-12, 0, 0))
	unassigned := createYouthAttendee(t, store, "Leo", time.Now().AddDate(-15, 0, 0))
	signUpAttendee(t, store, evt.ID, tented.ID)
	signUpAttendee(t, store, evt.ID, unassigned.ID)

	tent, err := store.Event.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("CreateTent: %v", err)
	}
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tented.ID); err != nil {
		t.Fatalf("AssignTentMember: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()

	for _, want := range []string{
		"Tenting Assignments",
		"Tent 1",
		"Mia Scout",
		"Unassigned",
		"Leo Scout",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in roster body:\n%s", want, body)
		}
	}
}

func TestRosterPage_RendersDrivers(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()
	evt := rosterEvent(t, store)

	driver := createAdultAttendee(t, store, "Dana", time.Now().AddDate(-40, 0, 0))
	signUpAttendee(t, store, evt.ID, driver.ID)
	if err := store.Event.AddDriver(ctx, evt.ID, driver.ID, 6); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()

	for _, want := range []string{
		"Drivers",
		"Dana Parent",
		"6",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in roster body:\n%s", want, body)
		}
	}
}

func TestRosterPage_OnlyRendersEnabledSections(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	ctx := t.Context()
	evt := &event.Event{
		Title:     "Basic Campout",
		Location:  "Lake",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := store.Event.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "Attendance (Roster)") {
		t.Errorf("expected Attendance section:\n%s", body)
	}
	for _, hidden := range []string{
		"Cooking Patrols",
		"Tenting Assignments",
		"Drivers",
	} {
		if strings.Contains(body, hidden) {
			t.Errorf("did not expect %q in roster body:\n%s", hidden, body)
		}
	}
}

type failingRosterRepo struct {
	event.Repository
	failAttendees        bool
	failResponsibilities bool
	failDrivers          bool
	failCookingPatrols   bool
	failTents            bool
	failSeatbeltSummary  bool
}

func (r *failingRosterRepo) GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error) {
	if r.failAttendees {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetAttendees(ctx, eventID)
}

func (r *failingRosterRepo) GetResponsibilities(ctx context.Context, eventID string) ([]event.ResponsibilityAssignment, error) {
	if r.failResponsibilities {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetResponsibilities(ctx, eventID)
}

func (r *failingRosterRepo) GetDrivers(ctx context.Context, eventID string) ([]event.DriverResponsibility, error) {
	if r.failDrivers {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetDrivers(ctx, eventID)
}

func (r *failingRosterRepo) ListCookingPatrols(ctx context.Context, eventID string) ([]*event.CookingPatrol, error) {
	if r.failCookingPatrols {
		return nil, errors.New("injected error")
	}
	return r.Repository.ListCookingPatrols(ctx, eventID)
}

func (r *failingRosterRepo) ListTents(ctx context.Context, eventID string) ([]*event.Tent, error) {
	if r.failTents {
		return nil, errors.New("injected error")
	}
	return r.Repository.ListTents(ctx, eventID)
}

func (r *failingRosterRepo) GetSeatbeltSummary(ctx context.Context, eventID string) (*event.SeatbeltSummary, error) {
	if r.failSeatbeltSummary {
		return nil, errors.New("injected error")
	}
	return r.Repository.GetSeatbeltSummary(ctx, eventID)
}

func TestRosterPage_RepoErrorsReturn500(t *testing.T) {
	tests := []struct {
		name string
		repo *failingRosterRepo
	}{
		{name: "GetAttendees", repo: &failingRosterRepo{failAttendees: true}},
		{name: "GetResponsibilities", repo: &failingRosterRepo{failResponsibilities: true}},
		{name: "GetDrivers", repo: &failingRosterRepo{failDrivers: true}},
		{name: "GetCookingPatrols", repo: &failingRosterRepo{failCookingPatrols: true}},
		{name: "GetTents", repo: &failingRosterRepo{failTents: true}},
		{name: "GetSeatbeltSummary", repo: &failingRosterRepo{failSeatbeltSummary: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, authService, store, _ := setupEventTest(t)
			evt := rosterEvent(t, store)
			tt.repo.Repository = store.Event
			handler := NewEventHandler(tt.repo, authService, store.RBAC, store.Profile, store.ParentYouthLink, appconfig.NewInMemoryRepository())

			req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/roster?id="+evt.ID)
			rr := httptest.NewRecorder()

			handler.RosterPage(rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestRosterPage_MissingEventIDReturns400(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)

	req := loggedInRequest(t, authService, "GET", "/events//roster?id=")
	rr := httptest.NewRecorder()

	handler.RosterPage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
