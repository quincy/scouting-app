package postgres

import (
	"context"
	"database/sql"
	"testing"

	"scout-app/internal/domain/profile"
)

func TestPostgresEventRepository_CreateTent_AutoNames(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Campout")

	first, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("CreateTent first: %v", err)
	}
	second, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("CreateTent second: %v", err)
	}

	if first.Name != "Tent 1" {
		t.Errorf("expected first tent name 'Tent 1', got %q", first.Name)
	}
	if second.Name != "Tent 2" {
		t.Errorf("expected second tent name 'Tent 2', got %q", second.Name)
	}
}

func TestPostgresEventRepository_CreateTent_NumberingGapsNeverReused(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Gappy Campout")

	first, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := repo.CreateTent(ctx, evt.ID); err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := repo.CreateTent(ctx, evt.ID); err != nil {
		t.Fatalf("create third: %v", err)
	}
	if err := repo.DeleteTent(ctx, first.ID); err != nil {
		t.Fatalf("delete first: %v", err)
	}

	next, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create after delete: %v", err)
	}
	if next.Name != "Tent 4" {
		t.Errorf("expected 'Tent 4' after gap, got %q", next.Name)
	}
}

func TestPostgresEventRepository_CreateTent_NumberingPerEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evtA := createTestEvent(t, repo, "Event A Campout")
	evtB := createTestEvent(t, repo, "Event B Campout")

	a, err := repo.CreateTent(ctx, evtA.ID)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := repo.CreateTent(ctx, evtB.ID)
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if a.Name != "Tent 1" {
		t.Errorf("expected 'Tent 1' for A, got %q", a.Name)
	}
	if b.Name != "Tent 1" {
		t.Errorf("expected 'Tent 1' for B, got %q", b.Name)
	}
}

func TestPostgresEventRepository_AssignTentMember_ListWithNames(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Assign Campout")

	scout := createTestProfile(t, profileRepo, "Taylor", "Smith", profile.MemberTypeYouth)
	other := createTestProfile(t, profileRepo, "Cameron", "Jones", profile.MemberTypeYouth)
	for _, id := range []string{scout.ID, other.ID} {
		if err := repo.SignUp(ctx, evt.ID, id); err != nil {
			t.Fatalf("sign up %s: %v", id, err)
		}
	}

	tent, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create tent: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err != nil {
		t.Fatalf("assign scout: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, other.ID); err != nil {
		t.Fatalf("assign other: %v", err)
	}

	tents, err := repo.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list tents: %v", err)
	}
	if len(tents) != 1 {
		t.Fatalf("expected 1 tent, got %d", len(tents))
	}
	members := tents[0].Members
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	names := map[string]string{}
	for _, m := range members {
		names[m.ProfileID] = m.ProfileName
	}
	if names[scout.ID] != "Taylor Smith" {
		t.Errorf("expected display name 'Taylor Smith', got %q", names[scout.ID])
	}
	if names[other.ID] != "Cameron Jones" {
		t.Errorf("expected display name 'Cameron Jones', got %q", names[other.ID])
	}
}

func TestPostgresEventRepository_AssignTentMember_MovesBetweenTents(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Move Campout")

	scout := createTestProfile(t, profileRepo, "Rowan", "Davis", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}

	tentA, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	tentB, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tentA.ID, scout.ID); err != nil {
		t.Fatalf("assign to A: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tentB.ID, scout.ID); err != nil {
		t.Fatalf("assign to B: %v", err)
	}

	tents, err := repo.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list tents: %v", err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected tent A to have 0 members, got %d", len(tents[0].Members))
	}
	if len(tents[1].Members) != 1 || tents[1].Members[0].ProfileID != scout.ID {
		t.Errorf("expected scout moved to tent B, got %+v", tents[1].Members)
	}
}

func TestPostgresEventRepository_RemoveTentMember(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Remove Campout")

	scout := createTestProfile(t, profileRepo, "Alex", "Wilson", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	tent, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create tent: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.RemoveTentMember(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	tents, err := repo.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list tents: %v", err)
	}
	if len(tents[0].Members) != 0 {
		t.Errorf("expected 0 members after remove, got %d", len(tents[0].Members))
	}
}

func TestPostgresEventRepository_DeleteTent_CascadesMembers(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Delete Tent")

	scout := createTestProfile(t, profileRepo, "Finley", "Brooks", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	tent, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create tent: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.DeleteTent(ctx, tent.ID); err != nil {
		t.Fatalf("delete tent: %v", err)
	}

	tents, err := repo.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list tents: %v", err)
	}
	if len(tents) != 0 {
		t.Errorf("expected no tents after delete, got %d", len(tents))
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err == nil {
		t.Error("expected assign to deleted tent to fail")
	}
}

func TestPostgresEventRepository_DeleteTent_NotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	if err := repo.DeleteTent(ctx, newUUID()); err == nil {
		t.Error("expected deleting a missing tent to fail")
	}
}

func TestPostgresEventRepository_ListTents_Empty(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Empty Campout")

	tents, err := repo.ListTents(ctx, evt.ID)
	if err != nil {
		t.Fatalf("list tents: %v", err)
	}
	if len(tents) != 0 {
		t.Errorf("expected no tents, got %d", len(tents))
	}
}

func TestTentRepository_DriverErrorPaths(t *testing.T) {
	badDB, err := sql.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/bad")
	if err != nil {
		t.Fatal(err)
	}
	defer badDB.Close()
	repo := NewEventRepository(badDB)
	ctx := context.Background()

	if _, err := repo.ListTents(ctx, "event-1"); err == nil {
		t.Error("expected ListTents to fail with unreachable DB")
	}
	if err := repo.DeleteTent(ctx, "tent-1"); err == nil {
		t.Error("expected DeleteTent to fail with unreachable DB")
	}
	if _, err := repo.CreateTent(ctx, "event-1"); err == nil {
		t.Error("expected CreateTent to fail with unreachable DB")
	}
	if err := repo.AssignTentMember(ctx, "event-1", "tent-1", "profile-1"); err == nil {
		t.Error("expected AssignTentMember to fail with unreachable DB")
	}
	if err := repo.RemoveTentMember(ctx, "event-1", "profile-1"); err == nil {
		t.Error("expected RemoveTentMember to fail with unreachable DB")
	}
}

func TestPostgresEventRepository_AssignTentMember_NonAttendeeFails(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "No Attendee")

	scout := createTestProfile(t, profileRepo, "Riley", "Parker", profile.MemberTypeYouth)
	tent, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create tent: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err == nil {
		t.Error("expected assign of non-attendee to fail via FK")
	}
}

func TestPostgresEventRepository_AssignTentMember_CrossEventTentRejected(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evtA := createTestEvent(t, repo, "Event A Tent")
	evtB := createTestEvent(t, repo, "Event B Tent")

	tentA, err := repo.CreateTent(ctx, evtA.ID)
	if err != nil {
		t.Fatalf("create tent in A: %v", err)
	}
	scout := createTestProfile(t, profileRepo, "Wren", "Diaz", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evtB.ID, scout.ID); err != nil {
		t.Fatalf("sign up to B: %v", err)
	}

	if err := repo.AssignTentMember(ctx, evtB.ID, tentA.ID, scout.ID); err == nil {
		t.Error("expected assigning an event-A tent via an event-B call to fail")
	}
}

func TestPostgresEventRepository_DeleteAttendee_WithTentMemberBlockedByFK(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "FK Restrict")

	scout := createTestProfile(t, profileRepo, "Skyler", "Reed", profile.MemberTypeYouth)
	if err := repo.SignUp(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	tent, err := repo.CreateTent(ctx, evt.ID)
	if err != nil {
		t.Fatalf("create tent: %v", err)
	}
	if err := repo.AssignTentMember(ctx, evt.ID, tent.ID, scout.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := testDB.ExecContext(ctx,
		`DELETE FROM event_attendees WHERE event_id = $1 AND profile_id = $2`,
		evt.ID, scout.ID,
	); err == nil {
		t.Error("expected deleting an attendee with a tent member to be blocked by FK (no hard cascade)")
	}

	if err := repo.RemoveTentMember(ctx, evt.ID, scout.ID); err != nil {
		t.Fatalf("remove tent member: %v", err)
	}
	if _, err := testDB.ExecContext(ctx,
		`DELETE FROM event_attendees WHERE event_id = $1 AND profile_id = $2`,
		evt.ID, scout.ID,
	); err != nil {
		t.Fatalf("expected attendee delete to succeed after removing tent member: %v", err)
	}
}

func TestPostgresEventRepository_CreateTent_QueryError(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()
	evt := createTestEvent(t, repo, "Schema Error")

	for _, stmt := range []string{
		"DROP TABLE IF EXISTS event_tent_members",
		"DROP TABLE IF EXISTS event_tents",
	} {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("schema drop %q: %v", stmt, err)
		}
	}
	defer func() {
		if _, err := testDB.Exec(`CREATE TABLE event_tents (
			id         UUID NOT NULL PRIMARY KEY,
			event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			name       TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
			t.Fatalf("recreate tents table: %v", err)
		}
		if _, err := testDB.Exec(`CREATE TABLE event_tent_members (
			event_id   UUID NOT NULL,
			profile_id UUID NOT NULL,
			tent_id    UUID NOT NULL REFERENCES event_tents(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (event_id, profile_id),
			FOREIGN KEY (event_id, profile_id) REFERENCES event_attendees(event_id, profile_id)
		)`); err != nil {
			t.Fatalf("recreate tent members table: %v", err)
		}
	}()

	if _, err := repo.CreateTent(ctx, evt.ID); err == nil {
		t.Error("expected CreateTent to fail after dropping table")
	}
}

func TestPostgresEventRepository_CreateTent_InsertError(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	if _, err := repo.CreateTent(ctx, newUUID()); err == nil {
		t.Error("expected CreateTent to fail when event does not exist (FK violation on insert)")
	}
}
