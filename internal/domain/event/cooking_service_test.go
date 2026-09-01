package event_test

import (
	"context"
	"errors"
	"testing"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/mock"
)

type fakeEventRepo struct {
	*mock.EventRepository
	createCookingPatrolErr error
	listCookingPatrolsErr  error
	clearCookingPatrolErr  error
}

func (f *fakeEventRepo) CreateCookingPatrol(ctx context.Context, eventID string, isAdult bool) (*event.CookingPatrol, error) {
	if f.createCookingPatrolErr != nil {
		return nil, f.createCookingPatrolErr
	}
	return f.EventRepository.CreateCookingPatrol(ctx, eventID, isAdult)
}

func (f *fakeEventRepo) ListCookingPatrols(ctx context.Context, eventID string) ([]*event.CookingPatrol, error) {
	if f.listCookingPatrolsErr != nil {
		return nil, f.listCookingPatrolsErr
	}
	return f.EventRepository.ListCookingPatrols(ctx, eventID)
}

func (f *fakeEventRepo) ClearCookingPatrolCook(ctx context.Context, patrolID string) error {
	if f.clearCookingPatrolErr != nil {
		return f.clearCookingPatrolErr
	}
	return f.EventRepository.ClearCookingPatrolCook(ctx, patrolID)
}

func (f *fakeEventRepo) AddDriver(ctx context.Context, eventID, profileID string, seatbeltCount int) error {
	return errors.New("unimplemented")
}
func (f *fakeEventRepo) RemoveDriver(ctx context.Context, eventID, profileID string) error {
	return errors.New("unimplemented")
}
func (f *fakeEventRepo) UpdateDriverSeatbeltCount(ctx context.Context, eventID, profileID string, seatbeltCount int) error {
	return errors.New("unimplemented")
}
func (f *fakeEventRepo) GetDrivers(ctx context.Context, eventID string) ([]event.DriverResponsibility, error) {
	return nil, errors.New("unimplemented")
}
func (f *fakeEventRepo) GetSeatbeltSummary(ctx context.Context, eventID string) (*event.SeatbeltSummary, error) {
	return nil, errors.New("unimplemented")
}
func (f *fakeEventRepo) ListUpcomingByProfileID(ctx context.Context, profileID string, limit, offset int) ([]*event.ListItem, error) {
	return nil, errors.New("unimplemented")
}
func (f *fakeEventRepo) ListPastByProfileID(ctx context.Context, profileID string, limit, offset int) ([]*event.ListItem, error) {
	return nil, errors.New("unimplemented")
}

func TestAfterSignUp_CookingEnabled_Adult_AutoJoin(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}

	adult := &profile.Profile{
		FirstName:  "Bob",
		LastName:   "Parent",
		MemberType: profile.MemberTypeAdult,
	}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patrols) != 1 {
		t.Fatalf("expected 1 patrol, got %d", len(patrols))
	}
	if patrols[0].Name != event.CookingPatrolAdultsName {
		t.Errorf("expected patrol name %q, got %q", event.CookingPatrolAdultsName, patrols[0].Name)
	}
	if !patrols[0].IsAdult {
		t.Error("expected patrol to be adult patrol")
	}
	if len(patrols[0].Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(patrols[0].Members))
	}
	if patrols[0].Members[0].ProfileID != adult.ID {
		t.Errorf("expected member %s, got %s", adult.ID, patrols[0].Members[0].ProfileID)
	}
}

func TestAfterSignUp_CookingDisabled_NoPatrol(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: false}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patrols) != 0 {
		t.Fatalf("expected 0 patrols, got %d", len(patrols))
	}
}

func TestAfterSignUp_Youth_NotPlaced(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patrols) != 0 {
		t.Fatalf("expected 0 patrols, got %d", len(patrols))
	}
}

func TestAfterSignUp_Adult_AutoJoinExistingPatrol_NoDup(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adultA := &profile.Profile{FirstName: "Ann", LastName: "A", MemberType: profile.MemberTypeAdult}
	adultB := &profile.Profile{FirstName: "Bob", LastName: "B", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adultA); err != nil {
		t.Fatal(err)
	}
	if err := profiles.Create(ctx, adultB); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adultA.ID); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adultB.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adultA.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AfterSignUp(ctx, evt.ID, adultB.ID); err != nil {
		t.Fatal(err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patrols) != 1 {
		t.Fatalf("expected 1 patrol, got %d", len(patrols))
	}
	adultPatrol := patrols[0]
	if len(adultPatrol.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(adultPatrol.Members))
	}
}

func TestAssignMember_AdultToYouthPatrol_Rejected(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}
	youthPatrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	err = svc.AssignMember(ctx, evt.ID, youthPatrol.ID, adult.ID)
	if err == nil {
		t.Fatal("expected error for adult assigned to youth patrol")
	}
	if !errors.Is(err, event.ErrAdultInYouthPatrol) {
		t.Errorf("expected ErrAdultInYouthPatrol, got %v", err)
	}
}

func TestAssignMember_AdultToAdultsPatrol_Allowed(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}
	adultPatrol, err := events.CreateCookingPatrol(ctx, evt.ID, true)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AssignMember(ctx, evt.ID, adultPatrol.ID, adult.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignMember_YouthToAdultsPatrol_Allowed(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	adultPatrol, err := events.CreateCookingPatrol(ctx, evt.ID, true)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AssignMember(ctx, evt.ID, adultPatrol.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignMember_YouthToYouthPatrol_Allowed(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	youthPatrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AssignMember(ctx, evt.ID, youthPatrol.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetCook_SetsMemberAsCook(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !patrols[0].Members[0].IsCook {
		t.Error("expected member to be cook")
	}
}

func TestSetCook_ReassignMovesCook(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	a := &profile.Profile{FirstName: "Ann", LastName: "A", MemberType: profile.MemberTypeYouth}
	b := &profile.Profile{FirstName: "Bob", LastName: "B", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := profiles.Create(ctx, b); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.ID, b.ID} {
		if err := events.SignUp(ctx, evt.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.ID, b.ID} {
		if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, id); err != nil {
			t.Fatal(err)
		}
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, b.ID); err != nil {
		t.Fatalf("expected reassign to succeed: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	cookCount := 0
	for _, m := range patrols[0].Members {
		if m.IsCook {
			cookCount++
			if m.ProfileID != b.ID {
				t.Errorf("expected cook to be %s, got %s", b.ID, m.ProfileID)
			}
		}
	}
	if cookCount != 1 {
		t.Errorf("expected exactly 1 cook, got %d", cookCount)
	}
}

func TestSetCook_NonMember_Error(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err == nil {
		t.Fatal("expected error setting non-member as cook")
	}
}

func TestAfterWithdraw_RemovesFromPatrol(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.AfterWithdraw(ctx, evt.ID, adult.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range patrols {
		for _, m := range p.Members {
			if m.ProfileID == adult.ID {
				t.Error("expected withdrawn attendee to be removed from patrol")
			}
		}
	}
}

func TestAfterWithdraw_ClearsCook(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.AfterWithdraw(ctx, evt.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range patrols[0].Members {
		if m.IsCook {
			t.Error("expected cook designation to be cleared on withdraw")
		}
	}
}

func TestClearCook_ClearsDesignation(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.ClearCook(ctx, evt.ID, patrol.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range patrols[0].Members {
		if m.IsCook {
			t.Error("expected cook designation to be cleared")
		}
	}
}

func TestAfterSignUp_EventNotFound_Error(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, "missing", "p1"); err == nil {
		t.Fatal("expected error for missing event")
	}
}

func TestAfterSignUp_ProfileNotFound_Error(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, "missing-profile"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestAfterSignUp_CreateAdultPatrolError_Propagates(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}
	events.createCookingPatrolErr = errors.New("create failed")

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adult.ID); err == nil {
		t.Fatal("expected error creating adult patrol")
	}
}

func TestAfterWithdraw_EventNotFound_Error(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterWithdraw(ctx, "missing", "p1"); err == nil {
		t.Fatal("expected error for missing event")
	}
}

func TestAssignMember_ProfileNotFound_Error(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AssignMember(ctx, evt.ID, patrol.ID, "missing-profile"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestAfterSignUp_ListAdultPatrolError_Propagates(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}
	events.listCookingPatrolsErr = errors.New("list failed")

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	adult := &profile.Profile{FirstName: "Bob", LastName: "Parent", MemberType: profile.MemberTypeAdult}
	if err := profiles.Create(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, adult.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.AfterSignUp(ctx, evt.ID, adult.ID); err == nil {
		t.Fatal("expected error listing cooking patrols")
	}
}

func TestAfterWithdraw_ClearCookError_Propagates(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	events.clearCookingPatrolErr = errors.New("clear failed")
	if err := svc.AfterWithdraw(ctx, evt.ID, youth.ID); err == nil {
		t.Fatal("expected error clearing cook")
	}
}

func TestCreatePatrol_CreatesYouthPatrol(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	patrol, err := svc.CreatePatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patrol.IsAdult {
		t.Error("expected a youth patrol")
	}
	if patrol.Name != event.CookingPatrolNextName(1) {
		t.Errorf("expected auto-name %q, got %q", event.CookingPatrolNextName(1), patrol.Name)
	}
}

func TestListPatrols_ReturnsPatrols(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	if _, err := events.CreateCookingPatrol(ctx, evt.ID, true); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	patrols, err := svc.ListPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patrols) != 1 {
		t.Fatalf("expected 1 patrol, got %d", len(patrols))
	}
	if !patrols[0].IsAdult {
		t.Error("expected the adult patrol")
	}
}

func TestDeletePatrol_RemovesPatrol(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.DeletePatrol(ctx, patrol.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patrols) != 0 {
		t.Errorf("expected patrol deleted, got %d patrols", len(patrols))
	}
}

func TestRemoveMember_RemovesMemberFromPatrol(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.RemoveMember(ctx, evt.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range patrols[0].Members {
		if m.ProfileID == youth.ID {
			t.Error("expected member to be removed")
		}
	}
}

func TestRemoveMember_ClearsCookWhenRemovingCook(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.RemoveMember(ctx, evt.ID, youth.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	patrols, err := events.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range patrols[0].Members {
		if m.IsCook {
			t.Error("expected cook designation to be cleared on member removal")
		}
	}
}

func TestRemoveMember_ListPatrolsError_Propagates(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}
	events.listCookingPatrolsErr = errors.New("list failed")

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}

	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.RemoveMember(ctx, evt.ID, "p1"); err == nil {
		t.Fatal("expected error listing cooking patrols")
	}
}

func TestRemoveMember_ClearCookError_Propagates(t *testing.T) {
	ctx := context.Background()
	profiles := mock.NewProfileRepository()
	events := &fakeEventRepo{EventRepository: mock.NewEventRepository(profiles)}

	evt := &event.Event{Title: "Campout", CookingEnabled: true}
	if err := events.Create(ctx, evt); err != nil {
		t.Fatal(err)
	}
	youth := &profile.Profile{FirstName: "Tim", LastName: "Scout", MemberType: profile.MemberTypeYouth}
	if err := profiles.Create(ctx, youth); err != nil {
		t.Fatal(err)
	}
	if err := events.SignUp(ctx, evt.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	patrol, err := events.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}
	svc := event.NewCookingPatrolService(events, profiles)
	if err := svc.SetCook(ctx, evt.ID, patrol.ID, youth.ID); err != nil {
		t.Fatal(err)
	}

	events.clearCookingPatrolErr = errors.New("clear failed")
	if err := svc.RemoveMember(ctx, evt.ID, youth.ID); err == nil {
		t.Fatal("expected error clearing cook")
	}
}
