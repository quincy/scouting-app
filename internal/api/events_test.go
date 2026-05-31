package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain"
	"scout-app/internal/storage/mock"
)

func futureEvent(id string, title string, daysFromNow int) *domain.Event {
	start := time.Now().AddDate(0, 0, daysFromNow)
	return &domain.Event{
		ID:        id,
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "campout",
		CreatedAt: time.Now(),
	}
}

func TestEventHandler_ListUpcomingPartial(t *testing.T) {
	repo := mock.NewEventRepository()
	handler := NewEventHandler(repo)

	repo.SeedEvents([]*domain.Event{
		futureEvent("f1", "Alpha", 1),
		futureEvent("f2", "Beta", 3),
	})

	req := httptest.NewRequest("GET", "/events/upcoming?offset=0", nil)
	rr := httptest.NewRecorder()

	handler.ListUpcoming(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListUpcoming returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	// Should contain the event cards
	if !strings.Contains(body, "Alpha") {
		t.Errorf("expected partial to contain 'Alpha', got:\n%s", body)
	}
	if !strings.Contains(body, "Beta") {
		t.Errorf("expected partial to contain 'Beta', got:\n%s", body)
	}

	// Should contain OOB counter update
	if !strings.Contains(body, "upcoming-count") {
		t.Errorf("expected partial to contain OOB counter, got:\n%s", body)
	}
	if !strings.Contains(body, "Showing 2 of 2") {
		t.Errorf("expected partial to say 'Showing 2 of 2', got:\n%s", body)
	}
}

func TestEventHandler_ListPastPartial(t *testing.T) {
	repo := mock.NewEventRepository()
	handler := NewEventHandler(repo)

	repo.SeedEvents([]*domain.Event{
		pastEvent("p1", "Old Meeting", 10),
		pastEvent("p2", "Recent Campout", 2),
	})

	req := httptest.NewRequest("GET", "/events/past?offset=0", nil)
	rr := httptest.NewRecorder()

	handler.ListPast(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListPast returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Old Meeting") {
		t.Errorf("expected partial to contain 'Old Meeting', got:\n%s", body)
	}
	if !strings.Contains(body, "Recent Campout") {
		t.Errorf("expected partial to contain 'Recent Campout', got:\n%s", body)
	}
	if !strings.Contains(body, "Showing 2 of 2") {
		t.Errorf("expected partial to say 'Showing 2 of 2', got:\n%s", body)
	}
}

func TestEventHandler_ListUpcoming_Pagination(t *testing.T) {
	repo := mock.NewEventRepository()
	handler := NewEventHandler(repo)

	// Seed 12 upcoming events (first page shows 10)
	var events []*domain.Event
	for i := 0; i < 12; i++ {
		events = append(events, futureEvent(
			fmt.Sprintf("f%d", i),
			fmt.Sprintf("Event %d", i),
			i+1,
		))
	}
	repo.SeedEvents(events)

	t.Run("first page returns 10 events with ShowMore", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/upcoming?offset=0", nil)
		rr := httptest.NewRecorder()
		handler.ListUpcoming(rr, req)

		body := rr.Body.String()
		if !strings.Contains(body, "Event 0") {
			t.Errorf("expected first event, got:\n%s", body)
		}
		if !strings.Contains(body, "Event 9") {
			t.Errorf("expected tenth event 'Event 9', got:\n%s", body)
		}
		if strings.Contains(body, "Event 10") {
			t.Error("did not expect Event 10 on first page")
		}

		// Should show "Showing 10 of 12"
		if !strings.Contains(body, "Showing 10 of 12") {
			t.Errorf("expected 'Showing 10 of 12', got:\n%s", body)
		}

		// Should have show-more button
		if !strings.Contains(body, "show-more-upcoming") {
			t.Errorf("expected show-more button to be present, got:\n%s", body)
		}
	})

	t.Run("second page returns remaining 2 events and hides button", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/upcoming?offset=10", nil)
		rr := httptest.NewRecorder()
		handler.ListUpcoming(rr, req)

		body := rr.Body.String()
		if !strings.Contains(body, "Event 10") {
			t.Errorf("expected Event 10 on second page, got:\n%s", body)
		}
		if !strings.Contains(body, "Event 11") {
			t.Errorf("expected Event 11 on second page, got:\n%s", body)
		}

		// Should show "Showing 12 of 12"
		if !strings.Contains(body, "Showing 12 of 12") {
			t.Errorf("expected 'Showing 12 of 12', got:\n%s", body)
		}
	})
}

func TestEventHandler_ListEvents_Empty(t *testing.T) {
	repo := mock.NewEventRepository()
	handler := NewEventHandler(repo)

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()

	handler.ListEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Showing 0 of 0") {
		t.Errorf("expected empty counts, got body:\n%s", body)
	}
}

func pastEvent(id string, title string, daysAgo int) *domain.Event {
	start := time.Now().AddDate(0, 0, -daysAgo)
	return &domain.Event{
		ID:        id,
		Title:     title,
		Location:  "Test Location",
		StartTime: start,
		EndTime:   start.Add(2 * time.Hour),
		Type:      "meeting",
		CreatedAt: time.Now(),
	}
}

func TestEventHandler_ListEvents(t *testing.T) {
	repo := mock.NewEventRepository()
	ctx := context.Background()
	handler := NewEventHandler(repo)

	// Seed one future and one past event
	repo.SeedEvents([]*domain.Event{
		futureEvent("f1", "Future Campout", 2),
		pastEvent("p1", "Past Meeting", 5),
	})

	// Sign up an attendee for the future event
	if err := repo.SignUp(ctx, "f1", "user-1"); err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()

	handler.ListEvents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListEvents returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	// Check content type is HTML
	ct := rr.Header().Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}

	// Verify page contains expected content
	if !strings.Contains(body, "Future Campout") {
		t.Errorf("expected page to contain 'Future Campout', got:\n%s", body)
	}
	if !strings.Contains(body, "Past Meeting") {
		t.Errorf("expected page to contain 'Past Meeting', got:\n%s", body)
	}
	if !strings.Contains(body, "campout") {
		t.Errorf("expected page to contain event type 'campout'")
	}
	if !strings.Contains(body, "1 attendee") {
		t.Errorf("expected page to contain '1 attendee'")
	}
	if !strings.Contains(body, "Upcoming Events") {
		t.Errorf("expected page to contain 'Upcoming Events' section")
	}
	if !strings.Contains(body, "Past Events") {
		t.Errorf("expected page to contain 'Past Events' section")
	}
}
