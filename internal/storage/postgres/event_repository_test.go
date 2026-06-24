package postgres

import (
	"context"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
)

func TestPostgresEventRepository_CreateAndGetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:       "Campout",
		Description: "Fun weekend",
		Location:    "Lake George",
		StartTime:   time.Now().Add(24 * time.Hour),
		EndTime:     time.Now().Add(48 * time.Hour),
		CostCents:   1500,
		Type:        "campout",
	}

	err := repo.Create(ctx, evt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if evt.ID == "" {
		t.Error("expected generated ID")
	}

	fetched, err := repo.GetByID(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if fetched.Title != "Campout" {
		t.Errorf("expected title 'Campout', got %q", fetched.Title)
	}
	if fetched.CostCents != 1500 {
		t.Errorf("expected cost 1500, got %d", fetched.CostCents)
	}
}

func TestPostgresEventRepository_SignUpAndWithdraw(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Test Event",
		Location:  "Test",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	p := &profile.Profile{
		FirstName:  "Attendee",
		LastName:   "One",
		Email:      "attendee1@test.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	attendees, err := eventRepo.GetAttendees(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees failed: %v", err)
	}
	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0].ID != p.ID {
		t.Errorf("expected attendee ID %s, got %s", p.ID, attendees[0].ID)
	}

	if err := eventRepo.Withdraw(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("Withdraw failed: %v", err)
	}

	attendees, err = eventRepo.GetAttendees(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetAttendees after withdraw failed: %v", err)
	}
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees after withdraw, got %d", len(attendees))
	}
}

func TestPostgresEventRepository_SignUp_Idempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Idempotent Test",
		Location:  "Test",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	eventRepo.Create(ctx, evt)

	p := &profile.Profile{
		FirstName: "Dup", LastName: "Test", Email: "dup@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	profileRepo.Create(ctx, p)

	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("first SignUp: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("second SignUp (idempotent) should not error: %v", err)
	}

	attendees, _ := eventRepo.GetAttendees(ctx, evt.ID)
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee after duplicate signup, got %d", len(attendees))
	}
}

func TestPostgresEventRepository_ListUpcomingAndPast(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	now := time.Now()

	past := &event.Event{Title: "Past", Location: "L", StartTime: now.Add(-48 * time.Hour), EndTime: now.Add(-24 * time.Hour), Type: "campout"}
	future1 := &event.Event{Title: "Alpha", Location: "L", StartTime: now.Add(24 * time.Hour), EndTime: now.Add(48 * time.Hour), Type: "campout"}
	future2 := &event.Event{Title: "Beta", Location: "L", StartTime: now.Add(72 * time.Hour), EndTime: now.Add(96 * time.Hour), Type: "campout"}

	for _, e := range []*event.Event{past, future1, future2} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("Create %s: %v", e.Title, err)
		}
	}

	upcoming, err := repo.ListUpcoming(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcoming: %v", err)
	}
	if len(upcoming) != 2 {
		t.Errorf("expected 2 upcoming, got %d", len(upcoming))
	}
	if len(upcoming) >= 2 && upcoming[0].Title != "Alpha" {
		t.Errorf("expected first upcoming to be Alpha, got %s", upcoming[0].Title)
	}

	pastEvents, err := repo.ListPast(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListPast: %v", err)
	}
	if len(pastEvents) != 1 {
		t.Errorf("expected 1 past event, got %d", len(pastEvents))
	}
}

func TestPostgresEventRepository_Update(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:       "Original",
		Description: "Original description",
		Location:    "Original location",
		StartTime:   time.Now().Add(24 * time.Hour),
		EndTime:     time.Now().Add(48 * time.Hour),
		CostCents:   1000,
		Type:        "campout",
		CreatedAt:   time.Now(),
	}
	if err := repo.Create(ctx, evt); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	createdAt := evt.CreatedAt

	updated := &event.Event{
		ID:          evt.ID,
		Title:       "Updated",
		Description: "Updated description",
		Location:    "Updated location",
		StartTime:   time.Now().Add(72 * time.Hour),
		EndTime:     time.Now().Add(96 * time.Hour),
		CostCents:   2000,
		Type:        "campout",
	}
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	fetched, err := repo.GetByID(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}

	if fetched.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", fetched.Title)
	}
	if fetched.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", fetched.Description)
	}
	if fetched.Location != "Updated location" {
		t.Errorf("expected location 'Updated location', got %q", fetched.Location)
	}
	if fetched.CostCents != 2000 {
		t.Errorf("expected CostCents 2000, got %d", fetched.CostCents)
	}
	if fetched.Type != "campout" {
		t.Errorf("expected type 'campout', got %q", fetched.Type)
	}
	if !fetched.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt should not change: original %v, got %v", createdAt, fetched.CreatedAt)
	}
	if fetched.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if fetched.UpdatedAt.Before(fetched.CreatedAt) {
		t.Error("UpdatedAt should not be before CreatedAt")
	}
}

func TestPostgresEventRepository_Delete(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "To Delete",
		Location:  "Somewhere",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := repo.Create(ctx, evt); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, evt.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.GetByID(ctx, evt.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestPostgresEventRepository_Delete_NonExistent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error deleting non-existent event, got nil")
	}
}

func TestPostgresEventRepository_Update_NonExistent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		ID:    "nonexistent-id",
		Title: "Ghost",
		Type:  "campout",
	}
	err := repo.Update(ctx, evt)
	if err == nil {
		t.Error("expected error updating non-existent event, got nil")
	}
}

func TestPostgresEventRepository_Pagination(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 5; i++ {
		e := &event.Event{
			Title:     "Event",
			Location:  "L",
			StartTime: now.Add(time.Duration(i+1) * 24 * time.Hour),
			EndTime:   now.Add(time.Duration(i+2) * 24 * time.Hour),
			Type:      "campout",
		}
		repo.Create(ctx, e)
	}

	results, err := repo.ListUpcoming(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListUpcoming limit=2: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with limit=2, got %d", len(results))
	}

	results, err = repo.ListUpcoming(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListUpcoming limit=2 offset=2: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with limit=2 offset=2, got %d", len(results))
	}

	results, err = repo.ListUpcoming(ctx, 2, 10)
	if err != nil {
		t.Fatalf("ListUpcoming offset beyond: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with offset beyond end, got %d", len(results))
	}
}
