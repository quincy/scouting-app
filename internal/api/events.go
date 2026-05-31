package api

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"scout-app/internal/domain"
)

//go:embed views/*.html
var viewsFS embed.FS

// EventHandler serves the event list page and HTMX partials.
type EventHandler struct {
	repo domain.EventRepository
	tmpl *template.Template
}

// eventsPageData is the data passed to layout.html for the initial page render.
type eventsPageData struct {
	UpcomingEvents     []*domain.EventListItem
	PastEvents         []*domain.EventListItem
	UpcomingDisplayed  int
	UpcomingTotal      int
	PastDisplayed      int
	PastTotal          int
	UpcomingNextOffset int
	PastNextOffset     int
	ShowMoreUpcoming   bool
	ShowMorePast       bool
}

// eventListPartialData is the data passed to event_list.html for HTMX partials.
type eventListPartialData struct {
	Events     []*domain.EventListItem
	Section    string // "upcoming" or "past"
	Displayed  int    // total displayed so far
	Total      int    // total events in this section
	NextOffset int    // offset for next request
	HasMore    bool   // whether more events are available
}

// NewEventHandler creates an EventHandler with compiled templates.
func NewEventHandler(repo domain.EventRepository) *EventHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &EventHandler{
		repo: repo,
		tmpl: tmpl,
	}
}

// ListEvents renders the full event list page with initial upcoming and past events.
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	upcomingEvents, err := h.repo.ListUpcoming(ctx, 10, 0)
	if err != nil {
		log.Printf("ListUpcoming: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	pastEvents, err := h.repo.ListPast(ctx, 1, 0)
	if err != nil {
		log.Printf("ListPast: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get total counts for "Showing X of Y"
	allUpcoming, err := h.repo.ListUpcoming(ctx, 100000, 0)
	if err != nil {
		log.Printf("ListUpcoming (all): %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	allPast, err := h.repo.ListPast(ctx, 100000, 0)
	if err != nil {
		log.Printf("ListPast (all): %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	upcomingTotal := len(allUpcoming)
	pastTotal := len(allPast)

	data := eventsPageData{
		UpcomingEvents:     upcomingEvents,
		PastEvents:         pastEvents,
		UpcomingDisplayed:  len(upcomingEvents),
		UpcomingTotal:      upcomingTotal,
		PastDisplayed:      len(pastEvents),
		PastTotal:          pastTotal,
		UpcomingNextOffset: 10,
		PastNextOffset:     1,
		ShowMoreUpcoming:   len(upcomingEvents) < upcomingTotal,
		ShowMorePast:       len(pastEvents) < pastTotal,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

// ListUpcoming handles HTMX partial requests for upcoming events.
func (h *EventHandler) ListUpcoming(w http.ResponseWriter, r *http.Request) {
	h.renderListPartial(w, r, "upcoming", h.repo.ListUpcoming)
}

// ListPast handles HTMX partial requests for past events.
func (h *EventHandler) ListPast(w http.ResponseWriter, r *http.Request) {
	h.renderListPartial(w, r, "past", h.repo.ListPast)
}

// listFunc matches the signature of EventRepository's ListUpcoming and ListPast.
type listFunc func(ctx context.Context, limit int, offset int) ([]*domain.EventListItem, error)

// renderListPartial renders the HTMX partial for a section of events.
func (h *EventHandler) renderListPartial(w http.ResponseWriter, r *http.Request, section string, fn listFunc) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	ctx := r.Context()
	events, err := fn(ctx, 10, offset)
	if err != nil {
		log.Printf("%s: %v", section, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get total count
	allEvents, err := fn(ctx, 100000, 0)
	if err != nil {
		log.Printf("%s (all): %v", section, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	total := len(allEvents)

	displayed := offset + len(events)
	nextOffset := offset + 10
	hasMore := displayed < total

	data := eventListPartialData{
		Events:     events,
		Section:    section,
		Displayed:  displayed,
		Total:      total,
		NextOffset: nextOffset,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_list.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}
