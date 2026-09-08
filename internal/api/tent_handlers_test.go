package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/postgres"
)

func setupTentMux() func() {
	orig := muxVars
	SetMuxVars(func(r *http.Request) map[string]string {
		return map[string]string{
			"id":         r.URL.Query().Get("id"),
			"tent_id":    r.URL.Query().Get("tent_id"),
			"profile_id": r.URL.Query().Get("profile_id"),
		}
	})
	return func() { muxVars = orig }
}

type failingTentRepo struct {
	event.Repository
	fail map[string]error
}

func (f *failingTentRepo) errFor(op string) error {
	if f.fail == nil {
		return nil
	}
	return f.fail[op]
}

func (f *failingTentRepo) CreateTent(ctx context.Context, eventID string) (*event.Tent, error) {
	if err := f.errFor("create"); err != nil {
		return nil, err
	}
	return f.Repository.CreateTent(ctx, eventID)
}

func (f *failingTentRepo) DeleteTent(ctx context.Context, tentID string) error {
	if err := f.errFor("delete"); err != nil {
		return err
	}
	return f.Repository.DeleteTent(ctx, tentID)
}

func (f *failingTentRepo) ListTents(ctx context.Context, eventID string) ([]*event.Tent, error) {
	if err := f.errFor("listTents"); err != nil {
		return nil, err
	}
	return f.Repository.ListTents(ctx, eventID)
}

func (f *failingTentRepo) GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error) {
	if err := f.errFor("attendees"); err != nil {
		return nil, err
	}
	return f.Repository.GetAttendees(ctx, eventID)
}

func (f *failingTentRepo) RemoveTentMember(ctx context.Context, eventID, profileID string) error {
	if err := f.errFor("removeTentMember"); err != nil {
		return err
	}
	return f.Repository.RemoveTentMember(ctx, eventID, profileID)
}

func tentingEvent(t *testing.T, store *postgres.Store, tentingEnabled bool) *event.Event {
	t.Helper()
	evt := &event.Event{
		Title:          "Campout",
		Location:       "Lake",
		StartTime:      time.Now(),
		EndTime:        time.Now().Add(2 * time.Hour),
		Type:           "campout",
		TentingEnabled: tentingEnabled,
	}
	if err := store.Event.Create(t.Context(), evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	return evt
}

func createTentForEvent(t *testing.T, store *postgres.Store, evtID string) *event.Tent {
	t.Helper()
	tent, err := store.Event.CreateTent(t.Context(), evtID)
	if err != nil {
		t.Fatalf("CreateTent: %v", err)
	}
	return tent
}

func TestEventHandler_EventDetail_RendersTentingTabWhenEnabled(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "tab-bar") {
		t.Errorf("expected tab bar on detail page:\n%s", body)
	}
	if !strings.Contains(body, `data-tab="tenting"`) {
		t.Errorf("expected Tenting tab on detail page:\n%s", body)
	}
	if !strings.Contains(body, `id="tent-section"`) {
		t.Errorf("expected tent section placeholder on detail page:\n%s", body)
	}
}

func TestEventTentingTab_SoloTentShowsAloneWarn(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "tent-alone-warn") {
		t.Errorf("expected alone-warn indicator for a solo tent:\n%s", body)
	}
	if !strings.Contains(body, "would sleep alone") {
		t.Errorf("expected alone-warn tooltip explaining the problem:\n%s", body)
	}
}

func TestEventTentingTab_TentTargetsExcludeInvalidTents(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)

	sue := createYouthProfile(t, store, "Sue")
	sue.Gender = "F"
	if err := store.Profile.Update(ctx, sue); err != nil {
		t.Fatalf("set Sue gender: %v", err)
	}
	signUpAttendee(t, store, evt.ID, sue.ID)

	validEmpty := createTentForEvent(t, store, evt.ID)

	validSameGender := createTentForEvent(t, store, evt.ID)
	amy := createYouthProfile(t, store, "Amy")
	amy.Gender = "F"
	if err := store.Profile.Update(ctx, amy); err != nil {
		t.Fatalf("set Amy gender: %v", err)
	}
	signUpAttendee(t, store, evt.ID, amy.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, validSameGender.ID, amy.ID); err != nil {
		t.Fatalf("seed same-gender tent: %v", err)
	}

	invalidMixed := createTentForEvent(t, store, evt.ID)
	mark := createYouthProfile(t, store, "Mark")
	mark.Gender = "M"
	if err := store.Profile.Update(ctx, mark); err != nil {
		t.Fatalf("set Mark gender: %v", err)
	}
	signUpAttendee(t, store, evt.ID, mark.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, invalidMixed.ID, mark.ID); err != nil {
		t.Fatalf("seed mixed-gender tent: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `value="`+validEmpty.ID+`"`) {
		t.Errorf("expected empty tent offered as a valid assign target for Sue:\n%s", body)
	}
	if !strings.Contains(body, `value="`+validSameGender.ID+`"`) {
		t.Errorf("expected same-gender tent offered as a valid assign target for Sue:\n%s", body)
	}
	if strings.Contains(body, `value="`+invalidMixed.ID+`"`) {
		t.Errorf("mixed-gender tent must not be offered as an assign target:\n%s", body)
	}
}

func TestEventTentingTab_TentTargetsExcludeAgeGapTents(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)

	sue := createYouthProfile(t, store, "Sue")
	sue.Gender = "M"
	sue.Birthdate = time.Date(2014, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, sue); err != nil {
		t.Fatalf("set Sue birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, sue.ID)

	validEmpty := createTentForEvent(t, store, evt.ID)

	validSameAge := createTentForEvent(t, store, evt.ID)
	amy := createYouthProfile(t, store, "Amy")
	amy.Gender = "M"
	amy.Birthdate = time.Date(2014, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, amy); err != nil {
		t.Fatalf("set Amy birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, amy.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, validSameAge.ID, amy.ID); err != nil {
		t.Fatalf("seed same-age tent: %v", err)
	}

	invalidAgeGap := createTentForEvent(t, store, evt.ID)
	mark := createYouthProfile(t, store, "Mark")
	mark.Gender = "M"
	mark.Birthdate = time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, mark); err != nil {
		t.Fatalf("set Mark birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, mark.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, invalidAgeGap.ID, mark.ID); err != nil {
		t.Fatalf("seed age-gap tent: %v", err)
	}

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `value="`+validEmpty.ID+`"`) {
		t.Errorf("expected empty tent offered as a valid assign target for Sue:\n%s", body)
	}
	if !strings.Contains(body, `value="`+validSameAge.ID+`"`) {
		t.Errorf("expected same-age tent offered as a valid assign target for Sue:\n%s", body)
	}
	if strings.Contains(body, `value="`+invalidAgeGap.ID+`"`) {
		t.Errorf("age-gap tent must not be offered as an assign target:\n%s", body)
	}
}

func TestEventTentingTab_RendersAgeAndBirthdate(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)

	tim := createYouthProfile(t, store, "Tim")
	tim.Gender = "M"
	tim.Birthdate = time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, tim); err != nil {
		t.Fatalf("set Tim birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	uma := createYouthProfile(t, store, "Uma")
	uma.Gender = "M"
	uma.Birthdate = time.Date(2013, 2, 10, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, uma); err != nil {
		t.Fatalf("set Uma birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, uma.ID)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "May 1, 2008") {
		t.Errorf("expected Tim's formatted birthdate on tenting tab:\n%s", body)
	}
	if !strings.Contains(body, "Feb 10, 2013") {
		t.Errorf("expected Uma's formatted birthdate in unassigned youth:\n%s", body)
	}
	timAge := strconv.Itoa(event.AgeWholeYears(tim.Birthdate, startTime(t, store, evt.ID)))
	if !strings.Contains(body, ">"+timAge+"<") {
		t.Errorf("expected Tim's age %q on tenting tab:\n%s", timAge, body)
	}
	umaAge := strconv.Itoa(event.AgeWholeYears(uma.Birthdate, startTime(t, store, evt.ID)))
	if !strings.Contains(body, ">"+umaAge+"<") {
		t.Errorf("expected Uma's age %q in unassigned youth:\n%s", umaAge, body)
	}
}

func TestEventTentingTab_NonAdminShowsAgeNotBirthdate(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)

	tim := createYouthProfile(t, store, "Tim")
	tim.Gender = "M"
	tim.Birthdate = time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, tim); err != nil {
		t.Fatalf("set Tim birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	createParentUser(t, store)
	req := loggedInAs(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID, "parent@scout.local")
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if strings.Contains(body, "May 1, 2008") {
		t.Errorf("expected birthdate hidden from non-admin:\n%s", body)
	}
	if strings.Contains(body, "Birth date") {
		t.Errorf("expected no birth date column for non-admin:\n%s", body)
	}
	age := strconv.Itoa(event.AgeWholeYears(tim.Birthdate, startTime(t, store, evt.ID)))
	if !strings.Contains(body, ">"+age+"<") {
		t.Errorf("expected age %q visible to non-admin:\n%s", age, body)
	}
}

func startTime(t *testing.T, store *postgres.Store, eventID string) time.Time {
	t.Helper()
	saved, err := store.Event.GetByID(t.Context(), eventID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	return saved.StartTime
}

func TestTentBirthdateLabel(t *testing.T) {
	if got := tentBirthdateLabel(time.Time{}); got != "—" {
		t.Errorf("zero birthdate: got %q, want dash", got)
	}
	if got := tentBirthdateLabel(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)); got != "—" {
		t.Errorf("sentinel 1970 birthdate: got %q, want dash", got)
	}
	if got := tentBirthdateLabel(time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)); got != "May 1, 2008" {
		t.Errorf("normal birthdate: got %q, want formatted date", got)
	}
}

func TestTentAgeLabel(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if got := tentAgeLabel(time.Time{}, at); got != "—" {
		t.Errorf("zero birthdate: got %q, want dash", got)
	}
	if got := tentAgeLabel(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), at); got != "—" {
		t.Errorf("sentinel 1970 birthdate: got %q, want dash", got)
	}
	if got := tentAgeLabel(time.Date(2013, 2, 10, 0, 0, 0, 0, time.UTC), at); got != "13" {
		t.Errorf("normal birthdate: got %q, want 13", got)
	}
}

func TestEventTentingTab_ReturnsTentSection(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Tents") {
		t.Errorf("expected Tents heading:\n%s", body)
	}
	if !strings.Contains(body, "Create Tent") {
		t.Errorf("expected Create Tent button for admin:\n%s", body)
	}
}

func TestEventTentingTab_TentingDisabled_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, false)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("EventTentingTab returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventTentingTab_NotFound(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupTentMux()()

	req := loggedInRequest(t, authService, "GET", "/events/nonexistent/tab/tenting?id=nonexistent")
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("EventTentingTab returned %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestEventTentingTab_Unauthenticated(t *testing.T) {
	handler, _, _, _ := setupEventTest(t)
	defer setupTentMux()()

	req := httptest.NewRequest("GET", "/events/e1/tab/tenting?id=e1", nil)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("EventTentingTab returned %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestEventTentingTab_MissingID_Returns400(t *testing.T) {
	handler, authService, _, _ := setupEventTest(t)
	defer setupTentMux()()

	req := loggedInRequest(t, authService, "GET", "/events//tab/tenting")
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("EventTentingTab returned %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEventTentingTab_BuildError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"listTents": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"/tab/tenting?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventTentingTab(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("EventTentingTab returned %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestEventHandler_EventDetail_OmitsTentSectionWhenDisabled(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	evt := tentingEvent(t, store, false)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if strings.Contains(body, "tab-bar") {
		t.Errorf("expected no tab bar when no sections enabled:\n%s", body)
	}
	if strings.Contains(body, `data-tab="tenting"`) {
		t.Errorf("expected no Tenting tab when tenting disabled:\n%s", body)
	}
	if strings.Contains(body, `id="tent-section"`) {
		t.Errorf("expected no tent section when tenting disabled:\n%s", body)
	}
}

func TestTentCreate_CreatesTent(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.TentCreate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), event.TentNextName(1)) {
		t.Errorf("expected auto-named tent in body:\n%s", rr.Body.String())
	}
	tents, err := store.Event.ListTents(t.Context(), evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents) != 1 {
		t.Fatalf("expected 1 tent, got %d", len(tents))
	}
	if tents[0].Name != event.TentNextName(1) {
		t.Errorf("expected %q, got %q", event.TentNextName(1), tents[0].Name)
	}
}

func TestTentCreate_ForbiddenForNonManager(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	createParentUser(t, store)

	req := loggedInAs(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID, "parent@scout.local")
	rr := httptest.NewRecorder()

	handler.TentCreate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestTentCreate_TentingDisabled_Rejected(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, false)

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.TentCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentCreate_PastEvent_Rejected(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	parentProfile := createParentUser(t, store)
	parentProfile.MemberType = profile.MemberTypeAdult
	evt := &event.Event{
		Title:          "Past Campout",
		Location:       "Lake",
		StartTime:      time.Now().Add(-48 * time.Hour),
		EndTime:        time.Now().Add(-46 * time.Hour),
		Type:           "campout",
		TentingEnabled: true,
	}
	if err := store.Event.Create(t.Context(), evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.TentCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentDelete_RemovesTent(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	tent := createTentForEvent(t, store, evt.ID)

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents/"+tent.ID+"?id="+evt.ID+"&tent_id="+tent.ID)
	rr := httptest.NewRecorder()

	handler.TentDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	tents, err := store.Event.ListTents(t.Context(), evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents) != 0 {
		t.Errorf("expected tent deleted, got %d tents", len(tents))
	}
}

func TestTentDelete_MissingTentID_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.TentDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentAssignMember_AssignsYouth(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	bob := createYouthProfile(t, store, "Bob")
	signUpAttendee(t, store, evt.ID, tim.ID)
	signUpAttendee(t, store, evt.ID, bob.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {bob.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 2 {
		t.Errorf("expected 2 members, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_SoloPlacementAllowedWithoutOverrideDialog(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {tim.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if strings.Contains(rr.Body.String(), "Save Anyway") {
		t.Errorf("expected no override dialog for solo placement:\n%s", rr.Body.String())
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 1 || tents[0].Members[0].ProfileID != tim.ID {
		t.Errorf("expected Tim assigned alone without an override dialog, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_MixedGenderBlockedWithoutSave(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	sue := createYouthProfile(t, store, "Sue")
	tim.Gender = "M"
	sue.Gender = "F"
	if err := store.Profile.Update(ctx, tim); err != nil {
		t.Fatalf("set Tim gender: %v", err)
	}
	if err := store.Profile.Update(ctx, sue); err != nil {
		t.Fatalf("set Sue gender: %v", err)
	}
	signUpAttendee(t, store, evt.ID, tim.ID)
	signUpAttendee(t, store, evt.ID, sue.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {sue.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 1 || tents[0].Members[0].ProfileID != tim.ID {
		t.Errorf("expected mixed-gender placement rejected (no change), got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_AgeGapBlockedWithoutSave(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)

	old := createYouthProfile(t, store, "Oscar")
	old.Gender = "M"
	old.Birthdate = time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, old); err != nil {
		t.Fatalf("set Oscar birthdate: %v", err)
	}
	young := createYouthProfile(t, store, "Young")
	young.Gender = "M"
	young.Birthdate = time.Date(2014, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Profile.Update(ctx, young); err != nil {
		t.Fatalf("set Young birthdate: %v", err)
	}
	signUpAttendee(t, store, evt.ID, old.ID)
	signUpAttendee(t, store, evt.ID, young.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, old.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {young.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "years older") {
		t.Errorf("expected age-gap violation message, got: %s", rr.Body.String())
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 1 || tents[0].Members[0].ProfileID != old.ID {
		t.Errorf("expected age-gap placement rejected (no change), got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_AdultRejected(t *testing.T) {
	handler, authService, store, adminProfile := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	signUpAttendee(t, store, evt.ID, adminProfile.ID)
	tent := createTentForEvent(t, store, evt.ID)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {adminProfile.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentAssignMember_MissingParams_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"tent_id": {"x"}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentRemoveMember_RemovesMember(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)
	if err := store.Event.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents/members/"+tim.ID+"?id="+evt.ID+"&profile_id="+tim.ID)
	rr := httptest.NewRecorder()

	handler.TentRemoveMember(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected member removed, got %+v", tents[0].Members)
	}
}

func TestTentRemoveMember_MissingProfileID_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents/members?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.TentRemoveMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentCreate_RepoError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"create": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()
	handler.TentCreate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestTentCreate_RenderError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"listTents": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInRequest(t, authService, "POST", "/events/"+evt.ID+"/tents?id="+evt.ID)
	rr := httptest.NewRecorder()
	handler.TentCreate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestTentDelete_RepoError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	tent := createTentForEvent(t, store, evt.ID)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"delete": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents/"+tent.ID+"?id="+evt.ID+"&tent_id="+tent.ID)
	rr := httptest.NewRecorder()
	handler.TentDelete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestTentRemoveMember_RepoError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	youth := createYouthProfile(t, store, "Alice")
	signUpAttendee(t, store, evt.ID, youth.ID)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"removeTentMember": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInRequest(t, authService, "DELETE", "/events/"+evt.ID+"/tents/members/"+youth.ID+"?id="+evt.ID+"&profile_id="+youth.ID)
	rr := httptest.NewRecorder()
	handler.TentRemoveMember(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestTentAssignMember_RepoError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	youth := createYouthProfile(t, store, "Alice")
	signUpAttendee(t, store, evt.ID, youth.ID)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"listTents": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {youth.ID}, "tent_id": {"y"}}.Encode())
	rr := httptest.NewRecorder()
	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}
