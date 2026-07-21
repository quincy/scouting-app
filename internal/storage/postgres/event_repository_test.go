package postgres

import (
	"context"
	"fmt"
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

	createdAt := time.Now().Add(-time.Second).Truncate(time.Microsecond)
	evt := &event.Event{
		Title:       "Original",
		Description: "Original description",
		Location:    "Original location",
		StartTime:   time.Now().Add(24 * time.Hour),
		EndTime:     time.Now().Add(48 * time.Hour),
		CostCents:   1000,
		Type:        "campout",
		CreatedAt:   createdAt,
	}
	if err := repo.Create(ctx, evt); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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

func TestPostgresEventRepository_AddDriverAndGetDrivers(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	p := &profile.Profile{
		FirstName:  "Driver",
		LastName:   "One",
		Email:      "driver1@test.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if err := eventRepo.AddDriver(ctx, evt.ID, p.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	drivers, err := eventRepo.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].ProfileID != p.ID {
		t.Errorf("expected profile ID %s, got %s", p.ID, drivers[0].ProfileID)
	}
	if drivers[0].SeatbeltCount != 5 {
		t.Errorf("expected seatbelt count 5, got %d", drivers[0].SeatbeltCount)
	}
}

func TestPostgresEventRepository_RemoveDriver(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	p := &profile.Profile{
		FirstName: "Driver", LastName: "One", Email: "driver1@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, p.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	if err := eventRepo.RemoveDriver(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("RemoveDriver: %v", err)
	}

	drivers, err := eventRepo.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 0 {
		t.Errorf("expected 0 drivers after remove, got %d", len(drivers))
	}
}

func TestPostgresEventRepository_UpdateDriverSeatbeltCount(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	p := &profile.Profile{
		FirstName: "Driver", LastName: "One", Email: "driver1@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, p.ID); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, p.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	if err := eventRepo.UpdateDriverSeatbeltCount(ctx, evt.ID, p.ID, 7); err != nil {
		t.Fatalf("UpdateDriverSeatbeltCount: %v", err)
	}

	drivers, err := eventRepo.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].SeatbeltCount != 7 {
		t.Errorf("expected seatbelt count 7, got %d", drivers[0].SeatbeltCount)
	}
}

func TestPostgresEventRepository_GetSeatbeltSummary(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	driver := &profile.Profile{
		FirstName: "Driver", LastName: "One", Email: "driver1@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, driver); err != nil {
		t.Fatalf("Create driver profile: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, driver.ID); err != nil {
		t.Fatalf("SignUp driver: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, driver.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	summary, err := eventRepo.GetSeatbeltSummary(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetSeatbeltSummary: %v", err)
	}
	if summary.TotalSeatbelts != 5 {
		t.Errorf("expected TotalSeatbelts 5, got %d", summary.TotalSeatbelts)
	}
	if summary.RequiredSeatbelts != 1 {
		t.Errorf("expected RequiredSeatbelts 1, got %d", summary.RequiredSeatbelts)
	}
	if summary.Available != 5 {
		t.Errorf("expected Available 5, got %d", summary.Available)
	}
	if !summary.Sufficient {
		t.Errorf("expected Sufficient to be true (5 available >= 1 required)")
	}
}

func TestPostgresEventRepository_GetSeatbeltSummary_Insufficient(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	driver := &profile.Profile{
		FirstName: "Driver", LastName: "One", Email: "driver1@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, driver); err != nil {
		t.Fatalf("Create driver: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, driver.ID); err != nil {
		t.Fatalf("SignUp driver: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, driver.ID, 5); err != nil {
		t.Fatalf("AddDriver: %v", err)
	}

	for i := 0; i < 6; i++ {
		passenger := &profile.Profile{
			FirstName: "Passenger", LastName: fmt.Sprintf("Num%d", i), Email: fmt.Sprintf("passenger%d@test.com", i),
			MemberType: profile.MemberTypeYouth, Status: profile.StatusActive,
		}
		if err := profileRepo.Create(ctx, passenger); err != nil {
			t.Fatalf("Create passenger %d: %v", i, err)
		}
		if err := eventRepo.SignUp(ctx, evt.ID, passenger.ID); err != nil {
			t.Fatalf("SignUp passenger %d: %v", i, err)
		}
	}

	summary, err := eventRepo.GetSeatbeltSummary(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetSeatbeltSummary: %v", err)
	}
	if summary.TotalSeatbelts != 5 {
		t.Errorf("expected TotalSeatbelts 5, got %d", summary.TotalSeatbelts)
	}
	if summary.RequiredSeatbelts != 7 {
		t.Errorf("expected RequiredSeatbelts 7, got %d", summary.RequiredSeatbelts)
	}
	if summary.Available != 5 {
		t.Errorf("expected Available 5, got %d", summary.Available)
	}
	if summary.Sufficient {
		t.Errorf("expected Sufficient to be false (5 available < 7 required)")
	}
}

func TestPostgresEventRepository_AddDriver_MultipleDrivers(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	evt := &event.Event{
		Title:     "Campout",
		Location:  "Lake George",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(48 * time.Hour),
		Type:      "campout",
	}
	if err := eventRepo.Create(ctx, evt); err != nil {
		t.Fatalf("Create event: %v", err)
	}

	p1 := &profile.Profile{
		FirstName: "Driver", LastName: "One", Email: "driver1@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	p2 := &profile.Profile{
		FirstName: "Driver", LastName: "Two", Email: "driver2@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p1); err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	if err := profileRepo.Create(ctx, p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, p1.ID); err != nil {
		t.Fatalf("SignUp p1: %v", err)
	}
	if err := eventRepo.SignUp(ctx, evt.ID, p2.ID); err != nil {
		t.Fatalf("SignUp p2: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, p1.ID, 5); err != nil {
		t.Fatalf("AddDriver p1: %v", err)
	}
	if err := eventRepo.AddDriver(ctx, evt.ID, p2.ID, 3); err != nil {
		t.Fatalf("AddDriver p2: %v", err)
	}

	drivers, err := eventRepo.GetDrivers(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetDrivers: %v", err)
	}
	if len(drivers) != 2 {
		t.Fatalf("expected 2 drivers, got %d", len(drivers))
	}
	if drivers[0].SeatbeltCount+drivers[1].SeatbeltCount != 8 {
		t.Errorf("expected total seatbelt count 8, got %d+%d", drivers[0].SeatbeltCount, drivers[1].SeatbeltCount)
	}

	summary, err := eventRepo.GetSeatbeltSummary(ctx, evt.ID)
	if err != nil {
		t.Fatalf("GetSeatbeltSummary: %v", err)
	}
	if summary.TotalSeatbelts != 8 {
		t.Errorf("expected TotalSeatbelts 8, got %d", summary.TotalSeatbelts)
	}
}

func TestPostgresEventRepository_ListByProfileID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	eventRepo := NewEventRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	now := time.Now()

	past := &event.Event{Title: "Past Event", Location: "L", StartTime: now.Add(-48 * time.Hour), EndTime: now.Add(-24 * time.Hour), Type: "campout"}
	future := &event.Event{Title: "Future Event", Location: "L", StartTime: now.Add(24 * time.Hour), EndTime: now.Add(48 * time.Hour), Type: "campout"}
	other := &event.Event{Title: "Other Event", Location: "L", StartTime: now.Add(72 * time.Hour), EndTime: now.Add(96 * time.Hour), Type: "campout"}

	for _, e := range []*event.Event{past, future, other} {
		if err := eventRepo.Create(ctx, e); err != nil {
			t.Fatalf("Create %s: %v", e.Title, err)
		}
	}

	p := &profile.Profile{
		FirstName: "Test", LastName: "User", Email: "test@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	p2 := &profile.Profile{
		FirstName: "Other", LastName: "User", Email: "other@test.com",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, p2); err != nil {
		t.Fatalf("Create profile2: %v", err)
	}

	if err := eventRepo.SignUp(ctx, future.ID, p.ID); err != nil {
		t.Fatalf("SignUp future: %v", err)
	}
	if err := eventRepo.SignUp(ctx, past.ID, p.ID); err != nil {
		t.Fatalf("SignUp past: %v", err)
	}
	if err := eventRepo.SignUp(ctx, other.ID, p2.ID); err != nil {
		t.Fatalf("SignUp other: %v", err)
	}

	upcoming, err := eventRepo.ListUpcomingByProfileID(ctx, p.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcomingByProfileID: %v", err)
	}
	if len(upcoming) != 1 {
		t.Errorf("expected 1 upcoming for profile, got %d", len(upcoming))
	}
	if len(upcoming) > 0 && upcoming[0].Title != "Future Event" {
		t.Errorf("expected 'Future Event', got %q", upcoming[0].Title)
	}

	pastEvents, err := eventRepo.ListPastByProfileID(ctx, p.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListPastByProfileID: %v", err)
	}
	if len(pastEvents) != 1 {
		t.Errorf("expected 1 past for profile, got %d", len(pastEvents))
	}
	if len(pastEvents) > 0 && pastEvents[0].Title != "Past Event" {
		t.Errorf("expected 'Past Event', got %q", pastEvents[0].Title)
	}

	otherUpcoming, err := eventRepo.ListUpcomingByProfileID(ctx, p2.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcomingByProfileID p2: %v", err)
	}
	if len(otherUpcoming) != 1 {
		t.Errorf("expected 1 upcoming for p2, got %d", len(otherUpcoming))
	}

	noEvents, err := eventRepo.ListUpcomingByProfileID(ctx, p.ID, 10, 10)
	if err != nil {
		t.Fatalf("ListUpcomingByProfileID offset: %v", err)
	}
	if len(noEvents) != 0 {
		t.Errorf("expected 0 with offset, got %d", len(noEvents))
	}
}

func TestPostgresEventRepository_GetDrivers_CancelledContext(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetDrivers(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestPostgresEventRepository_GetSeatbeltSummary_CancelledContext(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewEventRepository(testDB)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetSeatbeltSummary(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}
