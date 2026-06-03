package mock

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
)

func futureEvent(id string, title string, daysFromNow int) *event.Event {
	start := time.Now().AddDate(0, 0, daysFromNow)
	return &event.Event{
		ID:        id,
		Title:     title,
		Location:  "Camp",
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
		Location:  "Camp",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func newTestEventRepo() (*ProfileRepository, *EventRepository) {
	profileRepo := NewProfileRepository()
	return profileRepo, NewEventRepository(profileRepo)
}

func createTestProfile(repo *ProfileRepository, firstName, lastName, email string) *profile.Profile {
	p := &profile.Profile{
		FirstName:  firstName,
		LastName:   lastName,
		Email:      email,
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := repo.Create(context.Background(), p); err != nil {
		panic(err)
	}
	return p
}

func TestEventRepository_ListUpcoming_SortedASC(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	repo.SeedEvents([]*event.Event{
		pastEvent("p1", "Past Event", 2),
		futureEvent("f1", "Alpha", 1),
		futureEvent("f2", "Beta", 3),
	})

	results, err := repo.ListUpcoming(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcoming failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 upcoming events, got %d", len(results))
	}

	if results[0].Title != "Alpha" || results[1].Title != "Beta" {
		t.Errorf("expected ASC order by StartTime: Alpha then Beta, got %q then %q",
			results[0].Title, results[1].Title)
	}

	for _, r := range results {
		if r.ID == "p1" {
			t.Error("past event should not appear in ListUpcoming")
		}
	}
}

func TestEventRepository_ListPast_SortedDESC(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	repo.SeedEvents([]*event.Event{
		futureEvent("f1", "Future Event", 1),
		pastEvent("p1", "Zeta", 10),
		pastEvent("p2", "Alpha", 5),
	})

	results, err := repo.ListPast(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListPast failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 past events, got %d", len(results))
	}

	if results[0].Title != "Alpha" || results[1].Title != "Zeta" {
		t.Errorf("expected DESC order by StartTime: Alpha then Zeta, got %q then %q",
			results[0].Title, results[1].Title)
	}

	for _, r := range results {
		if r.ID == "f1" {
			t.Error("future event should not appear in ListPast")
		}
	}
}

func TestEventRepository_ListUpcoming_Pagination(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	var events []*event.Event
	for i := 0; i < 5; i++ {
		events = append(events, futureEvent(fmt.Sprintf("f%d", i), fmt.Sprintf("Event %d", i), i+1))
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

	for i, e := range events {
		e.ID = fmt.Sprintf("f%d", i)
		e.Title = fmt.Sprintf("Event %d", i)
	}

	repo.SeedEvents(events)

	t.Run("limit 2 offset 0 returns first 2", func(t *testing.T) {
		results, err := repo.ListUpcoming(ctx, 2, 0)
		if err != nil {
			t.Fatalf("ListUpcoming failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Event 0" || results[1].Title != "Event 1" {
			t.Errorf("expected first 2 events in order, got %q and %q", results[0].Title, results[1].Title)
		}
	})

	t.Run("limit 2 offset 2 returns next 2", func(t *testing.T) {
		results, err := repo.ListUpcoming(ctx, 2, 2)
		if err != nil {
			t.Fatalf("ListUpcoming failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Event 2" || results[1].Title != "Event 3" {
			t.Errorf("expected next 2 events in order, got %q and %q", results[0].Title, results[1].Title)
		}
	})

	t.Run("limit 2 offset 4 returns last 1", func(t *testing.T) {
		results, err := repo.ListUpcoming(ctx, 2, 4)
		if err != nil {
			t.Fatalf("ListUpcoming failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Title != "Event 4" {
			t.Errorf("expected last event, got %q", results[0].Title)
		}
	})

	t.Run("limit 2 offset 10 returns empty", func(t *testing.T) {
		results, err := repo.ListUpcoming(ctx, 2, 10)
		if err != nil {
			t.Fatalf("ListUpcoming failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})
}

func TestEventRepository_AttendeeCount(t *testing.T) {
	profileRepo, repo := newTestEventRepo()
	ctx := context.Background()

	profileA := createTestProfile(profileRepo, "Alice", "Smith", "alice@test.com")
	profileB := createTestProfile(profileRepo, "Bob", "Jones", "bob@test.com")

	evt := futureEvent("evt1", "Campout", 1)
	repo.SeedEvents([]*event.Event{evt})

	if err := repo.SignUp(ctx, "evt1", profileA.ID); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}
	if err := repo.SignUp(ctx, "evt1", profileB.ID); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	results, err := repo.ListUpcoming(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcoming failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}

	if results[0].AttendeeCount != 2 {
		t.Errorf("expected AttendeeCount=2, got %d", results[0].AttendeeCount)
	}

	if err := repo.Withdraw(ctx, "evt1", profileA.ID); err != nil {
		t.Fatalf("Withdraw failed: %v", err)
	}

	results, err = repo.ListUpcoming(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListUpcoming failed: %v", err)
	}

	if results[0].AttendeeCount != 1 {
		t.Errorf("expected AttendeeCount=1 after withdraw, got %d", results[0].AttendeeCount)
	}
}

func TestEventRepository_GetAttendees_WithProfiles(t *testing.T) {
	profileRepo, repo := newTestEventRepo()
	ctx := context.Background()

	profileA := createTestProfile(profileRepo, "Alice", "Smith", "alice@test.com")
	profileB := createTestProfile(profileRepo, "Bob", "Jones", "bob@test.com")

	evt := futureEvent("evt1", "Campout", 1)
	repo.SeedEvents([]*event.Event{evt})

	if err := repo.SignUp(ctx, "evt1", profileA.ID); err != nil {
		t.Fatalf("SignUp profileA: %v", err)
	}
	if err := repo.SignUp(ctx, "evt1", profileB.ID); err != nil {
		t.Fatalf("SignUp profileB: %v", err)
	}

	attendees, err := repo.GetAttendees(ctx, "evt1")
	if err != nil {
		t.Fatalf("GetAttendees failed: %v", err)
	}

	if len(attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendees))
	}

	names := make(map[string]bool)
	for _, p := range attendees {
		names[p.FirstName] = true
	}
	if !names["Alice"] {
		t.Error("expected Alice in attendees")
	}
	if !names["Bob"] {
		t.Error("expected Bob in attendees")
	}
}

func TestEventRepository_GetAttendees_NonExistentEvent(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	_, err := repo.GetAttendees(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent event, got nil")
	}
}
