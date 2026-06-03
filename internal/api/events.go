package api

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/user"
)

//go:embed views/*.html
var viewsFS embed.FS

// EventHandler serves the event list page, detail page, and HTMX partials.
type EventHandler struct {
	repo event.Repository
	auth *auth.AuthService
	tmpl *template.Template
}

// eventsPageData is the data passed to layout.html for the initial page render.
type eventsPageData struct {
	Title              string
	UpcomingEvents     []*event.ListItem
	PastEvents         []*event.ListItem
	UpcomingDisplayed  int
	UpcomingTotal      int
	PastDisplayed      int
	PastTotal          int
	UpcomingNextOffset int
	PastNextOffset     int
	ShowMoreUpcoming   bool
	ShowMorePast       bool
}

// eventDetailData is the data passed to event_detail.html.
type eventDetailData struct {
	Title         string
	Event         *event.Event
	CostDisplay   string
	Attendees     []attendeeViewModel
	AttendeeCount int
	IsAttending   bool
	IsPast        bool
}

// attendeeViewModel represents an attendee in the detail page.
type attendeeViewModel struct {
	Email string
	IsYou bool
}

// signupButtonData is the data for the signup_button.html partial.
type signupButtonData struct {
	IsAttending bool
	EventID     string
	IsPast      bool
}

// attendeeListData is the data for the attendee_list.html partial.
type attendeeListData struct {
	Attendees     []attendeeViewModel
	AttendeeCount int
}

// eventListPartialData is the data passed to event_list.html for HTMX partials.
type eventListPartialData struct {
	Events     []*event.ListItem
	Section    string // "upcoming" or "past"
	Displayed  int    // total displayed so far
	Total      int    // total events in this section
	NextOffset int    // offset for next request
	HasMore    bool   // whether more events are available
}

// NewEventHandler creates an EventHandler with compiled templates.
func NewEventHandler(repo event.Repository, auth *auth.AuthService) *EventHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &EventHandler{
		repo: repo,
		auth: auth,
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
		Title:              "Events",
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
type listFunc func(ctx context.Context, limit int, offset int) ([]*event.ListItem, error)

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

// EventDetail renders the event detail page.
func (h *EventHandler) EventDetail(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	event, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get current user
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Build attendee view models
	attendeeVMs := make([]attendeeViewModel, len(attendees))
	isAttending := false
	for i, u := range attendees {
		vm := attendeeViewModel{Email: u.Email}
		if u.ID == currentUser.ID {
			vm.IsYou = true
			isAttending = true
		}
		attendeeVMs[i] = vm
	}

	// Check if event is past
	isPast := event.EndTime.Before(time.Now())

	// Format cost
	costDisplay := formatCost(event.CostCents)

	data := eventDetailData{
		Title:         "Events",
		Event:         event,
		CostDisplay:   costDisplay,
		Attendees:     attendeeVMs,
		AttendeeCount: len(attendees),
		IsAttending:   isAttending,
		IsPast:        isPast,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_detail.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

// SignUp handles HTMX sign-up request.
func (h *EventHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	event, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if event.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot sign up for a past event", http.StatusBadRequest)
		return
	}

	if err := h.repo.SignUp(ctx, eventID, currentUser.ID); err != nil {
		log.Printf("SignUp: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Re-fetch attendees for updated list
	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	attendeeVMs := buildAttendeeVMs(attendees, currentUser)

	// Render both partials
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Signup button (now shows Withdraw)
	if err := h.tmpl.ExecuteTemplate(w, "signup_button.html", signupButtonData{
		IsAttending: true,
		EventID:     eventID,
		IsPast:      false,
	}); err != nil {
		log.Printf("template execution (signup_button): %v", err)
	}

	// Attendee list OOB
	if err := h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
		Attendees:     attendeeVMs,
		AttendeeCount: len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}
}

// Withdraw handles HTMX withdraw request.
func (h *EventHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	event, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if event.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot withdraw from a past event", http.StatusBadRequest)
		return
	}

	if err := h.repo.Withdraw(ctx, eventID, currentUser.ID); err != nil {
		log.Printf("Withdraw: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Re-fetch attendees for updated list
	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	attendeeVMs := buildAttendeeVMs(attendees, currentUser)

	// Render both partials
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Signup button (now shows Sign Up)
	if err := h.tmpl.ExecuteTemplate(w, "signup_button.html", signupButtonData{
		IsAttending: false,
		EventID:     eventID,
		IsPast:      false,
	}); err != nil {
		log.Printf("template execution (signup_button): %v", err)
	}

	// Attendee list OOB
	if err := h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
		Attendees:     attendeeVMs,
		AttendeeCount: len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}
}

// muxVars is a variable so it can be overridden in tests.
var muxVars = func(r *http.Request) map[string]string {
	return map[string]string{} // default noop; production will set it
}

// SetMuxVars allows tests to set path variables.
func SetMuxVars(fn func(r *http.Request) map[string]string) {
	muxVars = fn
}

// formatCost converts cents to a dollar string, e.g. 1500 -> "15.00".
func formatCost(cents int) string {
	if cents == 0 {
		return "0.00"
	}
	dollars := cents / 100
	remainder := cents % 100
	return fmt.Sprintf("%d.%02d", dollars, remainder)
}

// buildAttendeeVMs creates attendee view models with the current user marked.
func buildAttendeeVMs(attendees []*user.User, currentUser *user.User) []attendeeViewModel {
	vms := make([]attendeeViewModel, len(attendees))
	for i, u := range attendees {
		vm := attendeeViewModel{Email: u.Email}
		if u.ID == currentUser.ID {
			vm.IsYou = true
		}
		vms[i] = vm
	}
	return vms
}
