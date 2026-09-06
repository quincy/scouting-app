package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestEventHandler_EventDetail_RendersTentSectionWhenEnabled(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	evt := tentingEvent(t, store, true)

	req := loggedInRequest(t, authService, "GET", "/events/"+evt.ID+"?id="+evt.ID)
	rr := httptest.NewRecorder()

	handler.EventDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Tents") {
		t.Errorf("expected tent section in detail body:\n%s", rr.Body.String())
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
	if strings.Contains(rr.Body.String(), "id=\"tent-section\"") {
		t.Errorf("expected no tent section when tenting disabled:\n%s", rr.Body.String())
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

func TestTentAssignMember_BlockedShowsOverrideDialog(t *testing.T) {
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
	if !strings.Contains(rr.Body.String(), "Save Anyway") {
		t.Errorf("expected override dialog in body:\n%s", rr.Body.String())
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected placement blocked, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_OverrideSaves(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	ctx := t.Context()
	evt := tentingEvent(t, store, true)
	tim := createYouthProfile(t, store, "Tim")
	signUpAttendee(t, store, evt.ID, tim.ID)
	tent := createTentForEvent(t, store, evt.ID)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/override?id="+evt.ID,
		url.Values{"profile_id": {tim.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()

	handler.TentAssignMemberOverride(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	tents, err := store.Event.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("ListTents: %v", err)
	}
	if len(tents[0].Members) != 1 || tents[0].Members[0].ProfileID != tim.ID {
		t.Errorf("expected Tim assigned after override, got %+v", tents[0].Members)
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

func TestTentAssignMemberOverride_MissingParams_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/override?id="+evt.ID,
		url.Values{"tent_id": {"x"}}.Encode())
	rr := httptest.NewRecorder()
	handler.TentAssignMemberOverride(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentAssignMemberOverride_TentingDisabled_Returns400(t *testing.T) {
	handler, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, false)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/override?id="+evt.ID,
		url.Values{"profile_id": {"x"}, "tent_id": {"y"}}.Encode())
	rr := httptest.NewRecorder()
	handler.TentAssignMemberOverride(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTentAssignMemberOverride_RepoError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	youth := createYouthProfile(t, store, "Alice")
	signUpAttendee(t, store, evt.ID, youth.ID)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"listTents": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/override?id="+evt.ID,
		url.Values{"profile_id": {youth.ID}, "tent_id": {"y"}}.Encode())
	rr := httptest.NewRecorder()
	handler.TentAssignMemberOverride(rr, req)

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

func TestTentAssignMember_ViolationsRenderError_Returns500(t *testing.T) {
	_, authService, store, _ := setupEventTest(t)
	defer setupTentMux()()
	evt := tentingEvent(t, store, true)
	youth := createYouthProfile(t, store, "Alice")
	signUpAttendee(t, store, evt.ID, youth.ID)
	tent := createTentForEvent(t, store, evt.ID)

	repo := &failingTentRepo{Repository: store.Event, fail: map[string]error{"attendees": errors.New("boom")}}
	handler := newEventHandlerWithRepo(repo, authService, store)

	req := loggedInBodyRequest(t, authService, "POST", "/events/"+evt.ID+"/tents/assign?id="+evt.ID,
		url.Values{"profile_id": {youth.ID}, "tent_id": {tent.ID}}.Encode())
	rr := httptest.NewRecorder()
	handler.TentAssignMember(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}
