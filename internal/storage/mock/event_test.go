package mock

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"scout-app/internal/domain"
)

func futureEvent(id string, title string, daysFromNow int) *domain.Event {
	start := time.Now().AddDate(0, 0, daysFromNow)
	return &domain.Event{
		ID:        id,
		Title:     title,
		Location:  "Camp",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func pastEvent(id string, title string, daysAgo int) *domain.Event {
	start := time.Now().AddDate(0, 0, -daysAgo)
	return &domain.Event{
		ID:        id,
		Title:     title,
		Location:  "Camp",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func TestEventRepository_ListUpcoming_SortedASC(t *testing.T) {
	repo := NewEventRepository()
	ctx := context.Background()

	// Seed events: one past, two future
	repo.SeedEvents([]*domain.Event{
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
	repo := NewEventRepository()
	ctx := context.Background()

	// Seed events: two past, one future
	repo.SeedEvents([]*domain.Event{
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
	repo := NewEventRepository()
	ctx := context.Background()

	// Seed 5 future events
	var events []*domain.Event
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
	repo := NewEventRepository()
	ctx := context.Background()

	evt := futureEvent("evt1", "Campout", 1)
	repo.SeedEvents([]*domain.Event{evt})

	// Sign up two users
	if err := repo.SignUp(ctx, "evt1", "user-a"); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}
	if err := repo.SignUp(ctx, "evt1", "user-b"); err != nil {
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
	if err := repo.Withdraw(ctx, "evt1", "user-a"); err != nil {
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
