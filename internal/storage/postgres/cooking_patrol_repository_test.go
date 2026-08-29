package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
)

func createTestEvent(t *testing.T, repo *EventRepository, title string) *event.Event {
	t.Helper()
	evt := &event.Event{
		Title:     title,
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := repo.Create(context.Background(), evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}
	return evt
}

func createTestProfile(t *testing.T, repo *ProfileRepository, firstName, lastName string, memberType profile.MemberType) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      firstName + "." + lastName + "@test.com",
		MemberType: memberType,
		Status:     profile.StatusActive,
	}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}
	return p
}

func TestPostgresEventRepository_CreateCookingPatrol_AutoNamesYouthPatrols(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Cookout")

	first, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("CreateCookingPatrol first: %v", err)
	}
	second, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("CreateCookingPatrol second: %v", err)
	}

	if first.Name != "Cooking 1" {
		t.Errorf("expected first patrol name 'Cooking 1', got %q", first.Name)
	}
	if second.Name != "Cooking 2" {
		t.Errorf("expected second patrol name 'Cooking 2', got %q", second.Name)
	}
	if first.IsAdult {
		t.Error("expected first patrol IsAdult false")
	}
}

func TestPostgresEventRepository_CreateCookingPatrol_AdultsFixedName(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Adult Cookout")

	adult, err := repo.CreateCookingPatrol(ctx, evt.ID, true)
	if err != nil {
		t.Fatalf("CreateCookingPatrol adult: %v", err)
	}
	if adult.Name != "Adults" {
		t.Errorf("expected adult patrol name 'Adults', got %q", adult.Name)
	}
	if !adult.IsAdult {
		t.Error("expected adult patrol IsAdult true")
	}

	if _, err := repo.CreateCookingPatrol(ctx, evt.ID, true); err == nil {
		t.Error("expected second adult patrol to fail with unique constraint")
	}
}

func TestPostgresEventRepository_CreateCookingPatrol_NumberingGapsNeverReused(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Gappy Cookout")

	first, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := repo.CreateCookingPatrol(ctx, evt.ID, false); err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := repo.CreateCookingPatrol(ctx, evt.ID, false); err != nil {
		t.Fatalf("create third: %v", err)
	}
	if err := repo.DeleteCookingPatrol(ctx, first.ID); err != nil {
		t.Fatalf("delete first: %v", err)
	}

	next, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create after delete: %v", err)
	}
	if next.Name != "Cooking 4" {
		t.Errorf("expected 'Cooking 4' after gap, got %q", next.Name)
	}
}

func TestPostgresEventRepository_CreateCookingPatrol_NumberingPerEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evtA := createTestEvent(t, repo, "Event A")
	evtB := createTestEvent(t, repo, "Event B")

	a, err := repo.CreateCookingPatrol(ctx, evtA.ID, false)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := repo.CreateCookingPatrol(ctx, evtB.ID, false)
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if a.Name != "Cooking 1" {
		t.Errorf("expected 'Cooking 1' for A, got %q", a.Name)
	}
	if b.Name != "Cooking 1" {
		t.Errorf("expected 'Cooking 1' for B, got %q", b.Name)
	}
}

func TestPostgresEventRepository_AssignCookingPatrolMember_ListWithNames(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Assign Cookout")

	scout := createTestProfile(t, profileRepo, "Taylor", "Smith", profile.MemberTypeYouth)
	other := createTestProfile(t, profileRepo, "Cameron", "Jones", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up scout: %v", err)
	}
	if err := repo.SignUp(ctx, evt.ID, other.ID); err != nil {
		t.Fatalf("sign up other: %v", err)
	}

	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign scout: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, other.ID); err != nil {
		t.Fatalf("assign other: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	if len(patrols) != 1 {
		t.Fatalf("expected 1 patrol, got %d", len(patrols))
	}
	members := patrols[0].Members
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	names := map[string]string{}
	for _, m := range members {
		names[m.ProfileID] = m.ProfileName
		if m.IsCook {
			t.Error("expected newly assigned member IsCook false")
		}
	}
	if names[scout.ID] != "Taylor Smith" {
		t.Errorf("expected display name 'Taylor Smith', got %q", names[scout.ID])
	}
}

func TestPostgresEventRepository_AssignCookingPatrolMember_MovesBetweenPatrols(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Move Cookout")

	scout := createTestProfile(t, profileRepo, "Rowan", "Davis", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}

	patrolA, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	patrolB, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrolA.ID, scout.ID); err != nil {
		t.Fatalf("assign to A: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrolB.ID, scout.ID); err != nil {
		t.Fatalf("assign to B: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	if len(patrols[0].Members) != 0 {
		t.Errorf("expected patrol A to have 0 members, got %d", len(patrols[0].Members))
	}
	if len(patrols[1].Members) != 1 || patrols[1].Members[0].ProfileID != scout.ID {
		t.Errorf("expected scout moved to patrol B, got %+v", patrols[1].Members)
	}
}

func TestPostgresEventRepository_RemoveCookingPatrolMember(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Withdraw Cookout")

	scout := createTestProfile(t, profileRepo, "Alex", "Wilson", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.RemoveCookingPatrolMember(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	if len(patrols[0].Members) != 0 {
		t.Errorf("expected 0 members after remove, got %d", len(patrols[0].Members))
	}
}

func TestPostgresEventRepository_SetCookingPatrolCook_SingleCookInvariant(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Single Cook")

	scoutA := createTestProfile(t, profileRepo, "Jamie", "Lee", profile.MemberTypeYouth)
	scoutB := createTestProfile(t, profileRepo, "Casey", "Morgan", profile.MemberTypeYouth)
	for _, id := range []string{scoutA.ID, scoutB.ID} {
		if err := repo.SignUp(ctx, evt.ID, id); err != nil {
			t.Fatalf("sign up %s: %v", id, err)
		}
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	for _, id := range []string{scoutA.ID, scoutB.ID} {
		if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, id); err != nil {
			t.Fatalf("assign %s: %v", id, err)
		}
	}

	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scoutA.ID); err != nil {
		t.Fatalf("set cook A: %v", err)
	}
	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scoutB.ID); err != nil {
		t.Fatalf("set cook B: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	cooks := 0
	cookID := ""
	for _, m := range patrols[0].Members {
		if m.IsCook {
			cooks++
			cookID = m.ProfileID
		}
	}
	if cooks != 1 {
		t.Fatalf("expected exactly 1 cook, got %d", cooks)
	}
	if cookID != scoutB.ID {
		t.Errorf("expected cook to be scout B after reassignment, got %s", cookID)
	}
}

func TestPostgresEventRepository_ClearCookingPatrolCook(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Clear Cook")

	scout := createTestProfile(t, profileRepo, "Dana", "Kerr", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("set cook: %v", err)
	}
	if err := repo.ClearCookingPatrolCook(ctx, patrol.ID); err != nil {
		t.Fatalf("clear cook: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	for _, m := range patrols[0].Members {
		if m.IsCook {
			t.Error("expected no cooks after clear")
		}
	}
}

func TestPostgresEventRepository_DeleteCookingPatrol_CascadesMembers(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Delete Patrol")

	scout := createTestProfile(t, profileRepo, "Finley", "Brooks", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.DeleteCookingPatrol(ctx, patrol.ID); err != nil {
		t.Fatalf("delete patrol: %v", err)
	}

	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err == nil {
		t.Error("expected assign to deleted patrol to fail")
	}

	patrol, err = repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("recreate patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("reassign after delete: %v", err)
	}
	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("set cook after standalone cook row freed by cascade: %v", err)
	}
}

func TestPostgresEventRepository_DeleteCookingPatrol_NotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	if err := repo.DeleteCookingPatrol(ctx, newUUID()); err == nil {
		t.Error("expected deleting a missing patrol to fail")
	}
}

func TestPostgresEventRepository_ListCookingPatrols_Empty(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Empty Cookout")

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	if len(patrols) != 0 {
		t.Errorf("expected no patrols, got %d", len(patrols))
	}
}

func TestCookingPatrolRepository_DriverErrorPaths(t *testing.T) {
	badDB, err := sql.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/bad")
	if err != nil {
		t.Fatal(err)
	}
	defer badDB.Close()
	repo := NewEventRepository(badDB)
	ctx := context.Background()

	if _, err := repo.ListCookingPatrols(ctx, "event-1"); err == nil {
		t.Error("expected ListCookingPatrols to fail with unreachable DB")
	}
	if err := repo.DeleteCookingPatrol(ctx, "patrol-1"); err == nil {
		t.Error("expected DeleteCookingPatrol to fail with unreachable DB")
	}
	if _, err := repo.CreateCookingPatrol(ctx, "event-1", false); err == nil {
		t.Error("expected CreateCookingPatrol to fail with unreachable DB")
	}
	if err := repo.SetCookingPatrolCook(ctx, "event-1", "patrol-1", "profile-1"); err == nil {
		t.Error("expected SetCookingPatrolCook to fail with unreachable DB")
	}
	if err := repo.AssignCookingPatrolMember(ctx, "event-1", "patrol-1", "profile-1"); err == nil {
		t.Error("expected AssignCookingPatrolMember to fail with unreachable DB")
	}
	if err := repo.RemoveCookingPatrolMember(ctx, "event-1", "profile-1"); err == nil {
		t.Error("expected RemoveCookingPatrolMember to fail with unreachable DB")
	}
	if err := repo.ClearCookingPatrolCook(ctx, "patrol-1"); err == nil {
		t.Error("expected ClearCookingPatrolCook to fail with unreachable DB")
	}
}

func TestPostgresEventRepository_CreateCookingPatrol_NameQueryError(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Schema Error")

	recreate := func(stmt string) {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("schema statement: %v", err)
		}
	}
	recreate("DROP TABLE IF EXISTS event_cooking_patrol_members")
	recreate("DROP TABLE IF EXISTS event_cooking_patrols")
	defer func() {
		_, err := testDB.Exec(`CREATE TABLE event_cooking_patrols (
			id         UUID NOT NULL PRIMARY KEY,
			event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			name       TEXT NOT NULL,
			is_adult   BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
		if err != nil {
			t.Fatalf("recreate patrols table: %v", err)
		}
		_, err = testDB.Exec(`CREATE UNIQUE INDEX uq_event_cooking_patrols_single_adult
			ON event_cooking_patrols (event_id)
			WHERE is_adult`)
		if err != nil {
			t.Fatalf("recreate adult index: %v", err)
		}
		recreateCookingPatrolMembersTable(t)
	}()

	if _, err := repo.CreateCookingPatrol(ctx, evt.ID, false); err == nil {
		t.Error("expected CreateCookingPatrol to fail after dropping tables")
	}
}

func TestPostgresEventRepository_ListCookingPatrols_MemberQueryError(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Members Schema Error")

	profileRepo := NewProfileRepository(testDB)
	scout := createTestProfile(t, profileRepo, "Quinn", "Adler", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := testDB.Exec("DROP TABLE event_cooking_patrol_members"); err != nil {
		t.Fatalf("drop members table: %v", err)
	}
	defer func() {
		recreateCookingPatrolMembersTable(t)
	}()

	if _, err := repo.ListCookingPatrols(ctx, evt.ID); err == nil {
		t.Error("expected ListCookingPatrols to fail when members table is missing")
	}
	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scout.ID); err == nil {
		t.Error("expected SetCookingPatrolCook to fail when members table is missing")
	}
}

func recreateCookingPatrolMembersTable(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`CREATE TABLE event_cooking_patrol_members (
		event_id   UUID NOT NULL,
		profile_id UUID NOT NULL,
		patrol_id  UUID NOT NULL,
		is_cook    BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (event_id, profile_id),
		FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id) ON DELETE CASCADE,
		FOREIGN KEY (patrol_id) REFERENCES event_cooking_patrols(id) ON DELETE CASCADE
	)`)
	if err != nil {
		t.Fatalf("recreate members table: %v", err)
	}
	_, err = testDB.Exec(`CREATE UNIQUE INDEX uq_event_cooking_patrol_single_cook
		ON event_cooking_patrol_members (patrol_id)
		WHERE is_cook`)
	if err != nil {
		t.Fatalf("recreate single-cook index: %v", err)
	}
}

func TestPostgresEventRepository_AssignCookingPatrolMember_NonAttendeeFails(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "No Attendee")

	scout := createTestProfile(t, profileRepo, "Riley", "Parker", profile.MemberTypeYouth)
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err == nil {
		t.Error("expected assign of non-attendee to fail via FK")
	}
}

func TestPostgresEventRepository_SetCookingPatrolCook_NonMemberFails(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "NonMember Cook")

	scout := createTestProfile(t, profileRepo, "Morgan", "Nguyen", profile.MemberTypeYouth)
	member := createTestProfile(t, profileRepo, "Jules", "Costa", profile.MemberTypeYouth)
	for _, id := range []string{scout.ID, member.ID} {
		if err := repo.SignUp(ctx, evt.ID, id); err != nil {
			t.Fatalf("sign up %s: %v", id, err)
		}
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, member.ID); err != nil {
		t.Fatalf("assign member: %v", err)
	}
	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, member.ID); err != nil {
		t.Fatalf("set member as cook: %v", err)
	}

	if err := repo.SetCookingPatrolCook(ctx, evt.ID, patrol.ID, scout.ID); err == nil {
		t.Error("expected setting cook on non-member to fail")
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	cooks := 0
	for _, m := range patrols[0].Members {
		if m.IsCook {
			cooks++
			if m.ProfileID != member.ID {
				t.Errorf("expected cook to remain %s after failed reassignment, got %s", member.ID, m.ProfileID)
			}
		}
	}
	if cooks != 1 {
		t.Errorf("expected cook to be preserved after failed reassignment, got %d cooks", cooks)
	}
}

func TestPostgresEventRepository_AssignCookingPatrolMember_CrossEventPatrolRejected(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evtA := createTestEvent(t, repo, "Event A Cookout")
	evtB := createTestEvent(t, repo, "Event B Cookout")

	patrolA, err := repo.CreateCookingPatrol(ctx, evtA.ID, false)
	if err != nil {
		t.Fatalf("create patrol in A: %v", err)
	}
	scout := createTestProfile(t, profileRepo, "Wren", "Diaz", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evtB.ID, scout.ID); err != nil {
		t.Fatalf("sign up to B: %v", err)
	}

	if err := repo.AssignCookingPatrolMember(ctx, evtB.ID, patrolA.ID, scout.ID); err == nil {
		t.Error("expected assigning an event-A patrol via an event-B call to fail")
	}

	patrolsB, err := repo.ListCookingPatrols(ctx, evtB.ID)
	if err != nil {
		t.Fatalf("list patrols in B: %v", err)
	}
	for _, p := range patrolsB {
		if len(p.Members) != 0 {
			t.Errorf("expected no members assigned in event B, got %d", len(p.Members))
		}
	}
}

func TestPostgresEventRepository_AssignCookingPatrolMember_CascadesOnAttendeeDelete(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Cascade Delete")

	scout := createTestProfile(t, profileRepo, "Skyler", "Reed", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	patrol, err := repo.CreateCookingPatrol(ctx, evt.ID, false)
	if err != nil {
		t.Fatalf("create patrol: %v", err)
	}
	if err := repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := testDB.ExecContext(ctx,
		`DELETE FROM event_attendees WHERE event_id = $1 AND profile_id = $2`,
		evt.ID, scout.ID,
	); err != nil {
		t.Fatalf("delete attendee: %v", err)
	}

	patrols, err := repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list patrols: %v", err)
	}
	if len(patrols[0].Members) != 0 {
		t.Errorf("expected members cascaded away on attendee delete, got %d", len(patrols[0].Members))
	}
}
