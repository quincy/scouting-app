package event_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/mock"
)

type fakeTentEventRepo struct {
	*mock.EventRepository
}

func (f *fakeTentEventRepo) AddDriver(ctx context.Context, eventID string, profileID string, seatbeltCount int) error {
	return errors.New("unimplemented")
}

func (f *fakeTentEventRepo) RemoveDriver(ctx context.Context, eventID, profileID string) error {
	return errors.New("unimplemented")
}

func (f *fakeTentEventRepo) UpdateDriverSeatbeltCount(ctx context.Context, eventID, profileID string, seatbeltCount int) error {
	return errors.New("unimplemented")
}

func (f *fakeTentEventRepo) GetDrivers(ctx context.Context, eventID string) ([]event.DriverResponsibility, error) {
	return nil, errors.New("unimplemented")
}

func (f *fakeTentEventRepo) GetSeatbeltSummary(ctx context.Context, eventID string) (*event.SeatbeltSummary, error) {
	return nil, errors.New("unimplemented")
}

func (f *fakeTentEventRepo) ListUpcomingByProfileID(ctx context.Context, profileID string, limit, offset int) ([]*event.ListItem, error) {
	return nil, errors.New("unimplemented")
}

func (f *fakeTentEventRepo) ListPastByProfileID(ctx context.Context, profileID string, limit, offset int) ([]*event.ListItem, error) {
	return nil, errors.New("unimplemented")
}

type tentEnv struct {
	events   *fakeTentEventRepo
	profiles *mock.ProfileRepository
	links    *mock.ParentYouthLinkRepository
	svc      *event.TentService
}

func setupTentEnv(t *testing.T) *tentEnv {
	t.Helper()
	profiles := mock.NewProfileRepository()
	events := &fakeTentEventRepo{EventRepository: mock.NewEventRepository(profiles)}
	links := mock.NewParentYouthLinkRepository()
	return &tentEnv{
		events:   events,
		profiles: profiles,
		links:    links,
		svc:      event.NewTentService(events, profiles, links),
	}
}

func (e *tentEnv) newEvent(t *testing.T) *event.Event {
	t.Helper()
	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake",
		StartTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		Type:      "campout",
	}
	if err := e.events.Create(context.Background(), evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	return evt
}

func (e *tentEnv) newYouth(t *testing.T, name, gender string, birthdate time.Time) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  name,
		LastName:   "Scout",
		Email:      strings.ToLower(name) + ".scout@scout.local",
		Gender:     gender,
		Birthdate:  birthdate,
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := e.profiles.Create(context.Background(), p); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}
	return p
}

func (e *tentEnv) newAdult(t *testing.T, name string) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  name,
		LastName:   "Parent",
		Email:      strings.ToLower(name) + ".parent@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := e.profiles.Create(context.Background(), p); err != nil {
		t.Fatalf("Create adult profile: %v", err)
	}
	return p
}

func (e *tentEnv) signUp(t *testing.T, evtID string, profileID string) {
	t.Helper()
	if err := e.events.SignUp(context.Background(), evtID, profileID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
}

func (e *tentEnv) createTent(t *testing.T, evtID string) *event.Tent {
	t.Helper()
	tent, err := e.svc.CreateTent(context.Background(), evtID)
	if err != nil {
		t.Fatalf("CreateTent: %v", err)
	}
	return tent
}

func (e *tentEnv) linkSiblings(t *testing.T, parent, youthA, youthB string) {
	t.Helper()
	for _, youth := range []string{youthA, youthB} {
		link := &parentyouthlink.ParentYouthConnection{
			ParentProfileID: parent,
			YouthProfileID:  youth,
			Status:          parentyouthlink.StatusApproved,
		}
		if err := e.links.Create(context.Background(), link); err != nil {
			t.Fatalf("Create link: %v", err)
		}
	}
}

var (
	tentOlderBorn = time.Date(2011, 3, 5, 0, 0, 0, 0, time.UTC) // 15 on 2026-06-01
	tentYoungBorn = time.Date(2018, 3, 5, 0, 0, 0, 0, time.UTC) // 8 on 2026-06-01
)

func hasViolation(violations []event.Violation, code event.ViolationCode) bool {
	for _, v := range violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func TestTentAssignMember_SoloPlacementAllowed(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	tent := env.createTent(t, evt.ID)

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, tim.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected no blocking violations for a solo placement, got %+v", violations)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents[0].Members) != 1 || tents[0].Members[0].ProfileID != tim.ID {
		t.Errorf("expected Tim assigned alone, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_ValidPlacementSaves(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	bob := env.newYouth(t, "Bob", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	env.signUp(t, evt.ID, bob.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, bob.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %+v", violations)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents[0].Members) != 2 {
		t.Errorf("expected 2 members, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_MixedGender_Blocked(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	sue := env.newYouth(t, "Sue", "F", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	env.signUp(t, evt.ID, sue.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, sue.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) == 0 || !hasViolation(violations, event.ViolationMixedGender) {
		t.Errorf("expected mixed-gender violation, got %+v", violations)
	}
}

func TestTentAssignMember_AgeGap_Blocked(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	old := env.newYouth(t, "Old", "M", tentOlderBorn)
	young := env.newYouth(t, "Young", "M", tentYoungBorn)
	env.signUp(t, evt.ID, old.ID)
	env.signUp(t, evt.ID, young.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, old.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, young.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) == 0 || !hasViolation(violations, event.ViolationAgeGap) {
		t.Errorf("expected age-gap violation, got %+v", violations)
	}
}

func TestTentAssignMember_SiblingsExemptFromAgeGap(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	parent := env.newAdult(t, "Pat")
	old := env.newYouth(t, "Old", "M", tentOlderBorn)
	young := env.newYouth(t, "Young", "M", tentYoungBorn)
	env.linkSiblings(t, parent.ID, old.ID, young.ID)
	env.signUp(t, evt.ID, old.ID)
	env.signUp(t, evt.ID, young.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, old.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, young.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected siblings to be exempt from age gap, got %+v", violations)
	}
}

func TestTentAssignMember_PendingLinkDoesNotExemptSibling(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	parent := env.newAdult(t, "Pat")
	old := env.newYouth(t, "Old", "M", tentOlderBorn)
	young := env.newYouth(t, "Young", "M", tentYoungBorn)
	pendingLink := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  old.ID,
		Status:          parentyouthlink.StatusPending,
	}
	if err := env.links.Create(context.Background(), pendingLink); err != nil {
		t.Fatalf("Create pending link: %v", err)
	}
	env.signUp(t, evt.ID, old.ID)
	env.signUp(t, evt.ID, young.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, old.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, young.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) == 0 || !hasViolation(violations, event.ViolationAgeGap) {
		t.Errorf("expected age-gap violation since links are pending, got %+v", violations)
	}
}

func TestTentAssignMember_ListLinksError_Propagates(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	old := env.newYouth(t, "Old", "M", tentOlderBorn)
	young := env.newYouth(t, "Young", "M", tentYoungBorn)
	env.signUp(t, evt.ID, old.ID)
	env.signUp(t, evt.ID, young.ID)
	tent := env.createTent(t, evt.ID)
	env.events.AssignTentMember(ctx, evt.ID, tent.ID, old.ID)

	env.svc = event.NewTentService(env.events, env.profiles, &failingLinksRepo{err: errors.New("list failed")})
	_, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, young.ID, 2)
	if err == nil {
		t.Fatal("expected error listing approved links")
	}
}

type failingLinksRepo struct {
	err error
}

func (f *failingLinksRepo) Create(ctx context.Context, conn *parentyouthlink.ParentYouthConnection) error {
	return nil
}

func (f *failingLinksRepo) GetByID(ctx context.Context, id string) (*parentyouthlink.ParentYouthConnection, error) {
	return nil, f.err
}

func (f *failingLinksRepo) ListAll(ctx context.Context) ([]*parentyouthlink.ParentYouthConnection, error) {
	return nil, f.err
}

func (f *failingLinksRepo) ListByParent(ctx context.Context, parentProfileID string) ([]*parentyouthlink.ParentYouthConnection, error) {
	return nil, f.err
}

func (f *failingLinksRepo) ListByYouth(ctx context.Context, youthProfileID string) ([]*parentyouthlink.ParentYouthConnection, error) {
	return nil, f.err
}

func (f *failingLinksRepo) ListByStatus(ctx context.Context, status parentyouthlink.Status) ([]*parentyouthlink.ParentYouthConnection, error) {
	return nil, f.err
}

func (f *failingLinksRepo) UpdateStatus(ctx context.Context, id string, status parentyouthlink.Status, approvedBy string) error {
	return nil
}

func TestTentAssignMember_AdultRejected(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	adult := env.newAdult(t, "Bob")
	env.signUp(t, evt.ID, adult.ID)
	tent := env.createTent(t, evt.ID)

	_, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, adult.ID, 2)
	if err == nil {
		t.Fatal("expected error for adult assigned to tent")
	}
	if !errors.Is(err, event.ErrAdultInTent) {
		t.Errorf("expected ErrAdultInTent, got %v", err)
	}
}

func TestTentAssignMember_AlreadyInTent_NoOp(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	bob := env.newYouth(t, "Bob", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	env.signUp(t, evt.ID, bob.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	violations, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, tim.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected no violations for re-assignment, got %+v", violations)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents[0].Members) != 1 {
		t.Errorf("expected no duplicate members, got %+v", tents[0].Members)
	}
}

func TestTentAssignMember_ProfileNotFound_Error(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tent := env.createTent(t, evt.ID)

	_, err := env.svc.AssignMember(ctx, evt.ID, tent.ID, "missing-profile", 2)
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestTentAssignMember_TentNotFound_Error(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)

	_, err := env.svc.AssignMember(ctx, evt.ID, "missing-tent", tim.ID, 2)
	if err == nil {
		t.Fatal("expected error for missing tent")
	}
}

func TestCreateTent_AutoNames(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)

	tent, err := env.svc.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tent.Name != event.TentNextName(1) {
		t.Errorf("expected %q, got %q", event.TentNextName(1), tent.Name)
	}

	second, err := env.svc.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Name != event.TentNextName(2) {
		t.Errorf("expected %q, got %q", event.TentNextName(2), second.Name)
	}
}

func TestDeleteTent_RemovesTent(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tent := env.createTent(t, evt.ID)

	if err := env.svc.DeleteTent(ctx, tent.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents) != 0 {
		t.Errorf("expected tent deleted, got %d tents", len(tents))
	}
}

func TestListTents_ReturnsTentsInNumberOrder(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	env.createTent(t, evt.ID)
	env.createTent(t, evt.ID)

	tents, err := env.svc.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tents) != 2 {
		t.Fatalf("expected 2 tents, got %d", len(tents))
	}
	if tents[0].Name != event.TentNextName(1) || tents[1].Name != event.TentNextName(2) {
		t.Errorf("expected tents in number order, got %q, %q", tents[0].Name, tents[1].Name)
	}
}

func TestRemoveMember_RemovesScoutFromTent(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	if err := env.svc.RemoveMember(ctx, evt.ID, tim.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected member removed, got %+v", tents[0].Members)
	}
}

func TestAfterWithdraw_RemovesScoutFromTent(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	tent := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, tent.ID, tim.ID); err != nil {
		t.Fatalf("seed AssignTentMember: %v", err)
	}

	if err := env.svc.AfterWithdraw(ctx, evt.ID, tim.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tents, err := env.events.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected withdrawn attendee removed from tent, got %+v", tents[0].Members)
	}
}

func TestValidTargetTents_OffersEmptyAndSameGender_ExcludesMixedGenderAndAgeGap(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	sue := env.newYouth(t, "Sue", "F", tentYoungBorn)
	env.signUp(t, evt.ID, sue.ID)

	emptyTent := env.createTent(t, evt.ID)

	sameGender := env.createTent(t, evt.ID)
	amy := env.newYouth(t, "Amy", "F", tentYoungBorn)
	env.signUp(t, evt.ID, amy.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, sameGender.ID, amy.ID); err != nil {
		t.Fatalf("seed same-gender: %v", err)
	}

	mixedGender := env.createTent(t, evt.ID)
	mark := env.newYouth(t, "Mark", "M", tentYoungBorn)
	env.signUp(t, evt.ID, mark.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, mixedGender.ID, mark.ID); err != nil {
		t.Fatalf("seed mixed-gender: %v", err)
	}

	ageGap := env.createTent(t, evt.ID)
	old := env.newYouth(t, "Old", "F", tentOlderBorn)
	env.signUp(t, evt.ID, old.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, ageGap.ID, old.ID); err != nil {
		t.Fatalf("seed age-gap: %v", err)
	}

	valid, err := env.svc.ValidTargetTents(ctx, evt.ID, sue.ID, "", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantIDs := map[string]bool{emptyTent.ID: true, sameGender.ID: true}
	gotIDs := map[string]bool{}
	for _, v := range valid {
		gotIDs[v.ID] = true
	}
	for id := range wantIDs {
		if !gotIDs[id] {
			t.Errorf("expected valid target tent %q, got %v", id, gotIDs)
		}
	}
	if gotIDs[mixedGender.ID] {
		t.Errorf("mixed-gender tent %q must not be a valid target, got %v", mixedGender.ID, gotIDs)
	}
	if gotIDs[ageGap.ID] {
		t.Errorf("age-gap tent %q must not be a valid target, got %v", ageGap.ID, gotIDs)
	}
}

func TestValidTargetTents_ExcludesCurrentTent(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)

	current := env.createTent(t, evt.ID)
	other := env.createTent(t, evt.ID)
	if err := env.events.AssignTentMember(ctx, evt.ID, current.ID, tim.ID); err != nil {
		t.Fatalf("seed current tent: %v", err)
	}

	valid, err := env.svc.ValidTargetTents(ctx, evt.ID, tim.ID, current.ID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, v := range valid {
		if v.ID == current.ID {
			t.Errorf("current tent %q must be excluded from valid targets, got %v", current.ID, valid)
		}
	}
	if len(valid) != 1 || valid[0].ID != other.ID {
		t.Errorf("expected only the other tent as a valid target, got %v", valid)
	}
}

type failingTentListRepo struct {
	*fakeTentEventRepo
}

func (f *failingTentListRepo) ListTents(ctx context.Context, eventID string) ([]*event.Tent, error) {
	return nil, errors.New("list tents failed")
}

func TestValidTargetTents_ProfileNotFound_Error(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)

	_, err := env.svc.ValidTargetTents(ctx, evt.ID, "missing-profile", "", 2)
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestValidTargetTents_ListLinksError_Propagates(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	env.createTent(t, evt.ID)

	env.svc = event.NewTentService(env.events, env.profiles, &failingLinksRepo{err: errors.New("list links failed")})
	_, err := env.svc.ValidTargetTents(ctx, evt.ID, tim.ID, "", 2)
	if err == nil {
		t.Fatal("expected error listing approved links")
	}
}

func TestValidTargetTents_ListTentsError_Propagates(t *testing.T) {
	env := setupTentEnv(t)
	ctx := context.Background()
	evt := env.newEvent(t)
	tim := env.newYouth(t, "Tim", "M", tentYoungBorn)
	env.signUp(t, evt.ID, tim.ID)
	env.createTent(t, evt.ID)

	env.svc = event.NewTentService(&failingTentListRepo{fakeTentEventRepo: env.events}, env.profiles, env.links)
	_, err := env.svc.ValidTargetTents(ctx, evt.ID, tim.ID, "", 2)
	if err == nil {
		t.Fatal("expected error listing tents")
	}
}
