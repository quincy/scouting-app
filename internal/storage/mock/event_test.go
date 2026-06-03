package mock

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/user"
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

// newTestEventRepo creates a fresh user repo and event repo for testing.
func newTestEventRepo() (*UserRepository, *EventRepository) {
	userRepo := NewUserRepository()
	return userRepo, NewEventRepository(userRepo)
}

func TestEventRepository_ListUpcoming_SortedASC(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	// Seed events: one past, two future
	repo.SeedEvents([]*event.Event{
		pastEvent("p1", "Past Event", 2),
		futureEvent("f1", "Alpha", 1), // tomorrow
		futureEvent("f2", "Beta", 3),  // 3 days from now
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

	// Verify past event is excluded
	for _, r := range results {
		if r.ID == "p1" {
			t.Error("past event should not appear in ListUpcoming")
		}
	}
}

func TestEventRepository_ListPast_SortedDESC(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	// Seed events: two past, one future
	repo.SeedEvents([]*event.Event{
		futureEvent("f1", "Future Event", 1),
		pastEvent("p1", "Zeta", 10), // 10 days ago
		pastEvent("p2", "Alpha", 5), // 5 days ago (more recent)
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

	// Verify future event is excluded
	for _, r := range results {
		if r.ID == "f1" {
			t.Error("future event should not appear in ListPast")
		}
	}
}

func TestEventRepository_ListUpcoming_Pagination(t *testing.T) {
	_, repo := newTestEventRepo()
	ctx := context.Background()

	// Seed 5 future events
	var events []*event.Event
	for i := 0; i < 5; i++ {
		events = append(events, futureEvent(fmt.Sprintf("f%d", i), fmt.Sprintf("Event %d", i), i+1))
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

	// Manually assign IDs to match expected ordering
	for i, e := range events {
		e.ID = fmt.Sprintf("f%d", i)
		e.Title = fmt.Sprintf("Event %d", i) // Event 0 is tomorrow (ASC first), Event 4 is 5 days out (ASC last)
	}
	// Re-sort by days to make sure they're in the right order
	// Actually the futureEvent uses daysFromNow, so Event 0 (i=0, daysFromNow=1) is the soonest
	// That's the correct ASC order: Event 0, Event 1, ..., Event 4

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
	userRepo, repo := newTestEventRepo()
	ctx := context.Background()

	// Create users
	userA := &user.User{Email: "alice@test.com"}
	userB := &user.User{Email: "bob@test.com"}
	if err := userRepo.Create(ctx, userA); err != nil {
		t.Fatalf("Create userA: %v", err)
	}
	if err := userRepo.Create(ctx, userB); err != nil {
		t.Fatalf("Create userB: %v", err)
	}

	evt := futureEvent("evt1", "Campout", 1)
	repo.SeedEvents([]*event.Event{evt})

	// Sign up two users
	if err := repo.SignUp(ctx, "evt1", userA.ID); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}
	if err := repo.SignUp(ctx, "evt1", userB.ID); err != nil {
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

	// Withdraw one user
	if err := repo.Withdraw(ctx, "evt1", userA.ID); err != nil {
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

func TestEventRepository_GetAttendees_WithUserRepo(t *testing.T) {
	userRepo := NewUserRepository()
	repo := NewEventRepository(userRepo)
	ctx := context.Background()

	// Create users
	userA := &user.User{Email: "alice@test.com"}
	userB := &user.User{Email: "bob@test.com"}
	if err := userRepo.Create(ctx, userA); err != nil {
		t.Fatalf("Create userA: %v", err)
	}
	if err := userRepo.Create(ctx, userB); err != nil {
		t.Fatalf("Create userB: %v", err)
	}

	evt := futureEvent("evt1", "Campout", 1)
	repo.SeedEvents([]*event.Event{evt})

	// Sign up users
	if err := repo.SignUp(ctx, "evt1", userA.ID); err != nil {
		t.Fatalf("SignUp userA: %v", err)
	}
	if err := repo.SignUp(ctx, "evt1", userB.ID); err != nil {
		t.Fatalf("SignUp userB: %v", err)
	}

	attendees, err := repo.GetAttendees(ctx, "evt1")
	if err != nil {
		t.Fatalf("GetAttendees failed: %v", err)
	}

	if len(attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendees))
	}

	// Verify emails match (order may vary)
	emails := make(map[string]bool)
	for _, u := range attendees {
		emails[u.Email] = true
	}
	if !emails["alice@test.com"] {
		t.Error("expected alice@test.com in attendees")
	}
	if !emails["bob@test.com"] {
		t.Error("expected bob@test.com in attendees")
	}
}

func TestEventRepository_GetAttendees_NonExistentEvent(t *testing.T) {
	userRepo := NewUserRepository()
	repo := NewEventRepository(userRepo)
	ctx := context.Background()

	_, err := repo.GetAttendees(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent event, got nil")
	}
}
