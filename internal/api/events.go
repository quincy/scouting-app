package api

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

//go:embed views/*.html
var viewsFS embed.FS

type EventHandler struct {
	repo            event.Repository
	auth            *auth.AuthService
	rbac            rbac.Repository
	profiles        profile.Repository
	parentYouthLink parentyouthlink.Repository
	tmpl            *template.Template
	unitType        string
	unitNumber      string
}

type eventsPageData struct {
	Title              string
	IsAdmin            bool
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

type profileSignUpVM struct {
	ProfileID   string
	ProfileName string
	IsAttending bool
}

type eventFormData struct {
	Title        string
	IsAdmin      bool
	Event        *event.Event
	Errors       map[string]string
	FlashSuccess string
	UnitType     string
	UnitNumber   string
}

type eventDetailData struct {
	Title           string
	IsAdmin         bool
	Event           *event.Event
	CostDisplay     string
	DescriptionHTML template.HTML
	FlashSuccess    string
	YouthAttendees  []attendeeViewModel
	YouthCount      int
	AdultAttendees  []attendeeViewModel
	AdultCount      int
	AttendeeCount   int
	Profiles        []profileSignUpVM
	IsPast          bool
}

type attendeeViewModel struct {
	ProfileName string
}

type signupSectionData struct {
	EventID  string
	IsPast   bool
	Profiles []profileSignUpVM
}

type attendeeListData struct {
	YouthAttendees []attendeeViewModel
	YouthCount     int
	AdultAttendees []attendeeViewModel
	AdultCount     int
	AttendeeCount  int
}

type eventListPartialData struct {
	Events     []*event.ListItem
	Section    string
	Displayed  int
	Total      int
	NextOffset int
	HasMore    bool
}

func NewEventHandler(repo event.Repository, auth *auth.AuthService, rbac rbac.Repository, profiles profile.Repository, parentYouthLink parentyouthlink.Repository, unitType, unitNumber string) *EventHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &EventHandler{
		repo:            repo,
		auth:            auth,
		rbac:            rbac,
		profiles:        profiles,
		parentYouthLink: parentYouthLink,
		tmpl:            tmpl,
		unitType:        unitType,
		unitNumber:      unitNumber,
	}
}

func (h *EventHandler) isAdmin(ctx context.Context, r *http.Request) bool {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		return false
	}
	perms, err := h.rbac.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if p.Name == "event:create" {
			return true
		}
	}
	return false
}

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

	pageTitle := fmt.Sprintf("%s %s Events", h.unitType, h.unitNumber)
	data := eventsPageData{
		Title:              pageTitle,
		IsAdmin:            h.isAdmin(ctx, r),
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

func (h *EventHandler) ListUpcoming(w http.ResponseWriter, r *http.Request) {
	h.renderListPartial(w, r, "upcoming", h.repo.ListUpcoming)
}

func (h *EventHandler) ListPast(w http.ResponseWriter, r *http.Request) {
	h.renderListPartial(w, r, "past", h.repo.ListPast)
}

type listFunc func(ctx context.Context, limit int, offset int) ([]*event.ListItem, error)

func (h *EventHandler) renderListPartial(w http.ResponseWriter, r *http.Request, section string, fn listFunc) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	ctx := r.Context()
	events, err := fn(ctx, 10, offset)
	if err != nil {
		log.Printf("%s: %v", section, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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

	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	isPast := event.EndTime.Before(time.Now())
	costDisplay := formatCost(event.CostCents)
	profileVMs := h.buildProfileSignUps(ctx, currentUser.ID, attendees)
	youthVMs, adultVMs := splitAttendeeVMs(attendees)

	renderedDesc, err := renderMarkdown(event.Description)
	if err != nil {
		renderedDesc = template.HTMLEscapeString(event.Description)
	}

	flashSuccess := ""
	if r.URL.Query().Get("created") == "1" {
		flashSuccess = "Event created successfully!"
	}

	detailTitle := fmt.Sprintf("%s %s Events", h.unitType, h.unitNumber)
	data := eventDetailData{
		Title:           detailTitle,
		IsAdmin:         h.isAdmin(ctx, r),
		Event:           event,
		CostDisplay:     costDisplay,
		DescriptionHTML: template.HTML(renderedDesc),
		FlashSuccess:    flashSuccess,
		YouthAttendees:  youthVMs,
		YouthCount:      len(youthVMs),
		AdultAttendees:  adultVMs,
		AdultCount:      len(adultVMs),
		AttendeeCount:   len(attendees),
		Profiles:        profileVMs,
		IsPast:          isPast,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_detail.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) EventCreateForm(w http.ResponseWriter, r *http.Request) {
	data := eventFormData{
		Title:      "Create Event",
		IsAdmin:    h.isAdmin(r.Context(), r),
		Event:      &event.Event{},
		UnitType:   h.unitType,
		UnitNumber: h.unitNumber,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_form.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) EventCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")
	costStr := r.FormValue("cost")
	eventType := r.FormValue("type")

	errors := make(map[string]string)

	if title == "" {
		errors["title"] = "Title is required"
	}
	if location == "" {
		errors["location"] = "Location is required"
	}
	if startTimeStr == "" {
		errors["start_time"] = "Start time is required"
	}
	if endTimeStr == "" {
		errors["end_time"] = "End time is required"
	}

	var costCents int
	if costStr == "" {
		errors["cost"] = "Cost is required"
	} else {
		costFloat, err := strconv.ParseFloat(costStr, 64)
		if err != nil {
			errors["cost"] = "Invalid cost value"
		} else if costFloat < 0 {
			errors["cost"] = "Cost must not be negative"
		} else {
			costCents = int(costFloat * 100)
		}
	}

	var startTime, endTime time.Time
	if startTimeStr != "" {
		var err error
		startTime, err = time.Parse("2006-01-02T15:04", startTimeStr)
		if err != nil {
			errors["start_time"] = "Invalid start time format"
		}
	}
	if endTimeStr != "" {
		var err error
		endTime, err = time.Parse("2006-01-02T15:04", endTimeStr)
		if err != nil {
			errors["end_time"] = "Invalid end time format"
		}
	}

	if len(errors) == 0 && !endTime.After(startTime) {
		errors["end_time"] = "End time must be after start time"
	}

	if len(errors) > 0 {
		data := eventFormData{
			Title:      "Create Event",
			IsAdmin:    h.isAdmin(r.Context(), r),
			Event:      &event.Event{},
			UnitType:   h.unitType,
			UnitNumber: h.unitNumber,
			Errors:     errors,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := h.tmpl.ExecuteTemplate(w, "event_form.html", data); err != nil {
			log.Printf("template execution: %v", err)
		}
		return
	}

	evt := &event.Event{
		Title:       title,
		Description: description,
		Location:    location,
		StartTime:   startTime,
		EndTime:     endTime,
		CostCents:   costCents,
		Type:        eventType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := r.Context()
	if err := h.repo.Create(ctx, evt); err != nil {
		log.Printf("EventCreate: %v", err)
		data := eventFormData{
			Title:      "Create Event",
			IsAdmin:    h.isAdmin(r.Context(), r),
			Event:      &event.Event{},
			UnitType:   h.unitType,
			UnitNumber: h.unitNumber,
			Errors:     map[string]string{"title": "Failed to create event"},
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := h.tmpl.ExecuteTemplate(w, "event_form.html", data); err != nil {
			log.Printf("template execution: %v", err)
		}
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/events/%s?created=1", evt.ID), http.StatusFound)
}

func (h *EventHandler) buildProfileSignUps(ctx context.Context, currentUserID string, attendees []*profile.Profile) []profileSignUpVM {
	var youthVMs []profileSignUpVM
	var adultVMs []profileSignUpVM

	userProfile, err := h.profiles.GetByUserID(ctx, currentUserID)
	if err == nil {
		isAttending := false
		for _, a := range attendees {
			if a.ID == userProfile.ID {
				isAttending = true
				break
			}
		}
		adultVMs = append(adultVMs, profileSignUpVM{
			ProfileID:   userProfile.ID,
			ProfileName: userProfile.DisplayName(),
			IsAttending: isAttending,
		})
	}

	if userProfile != nil {
		links, err := h.parentYouthLink.ListByParent(ctx, userProfile.ID)
		if err == nil {
			for _, link := range links {
				if link.Status != parentyouthlink.StatusApproved {
					continue
				}
				youthProfile, err := h.profiles.GetByID(ctx, link.YouthProfileID)
				if err != nil {
					continue
				}
				isAttending := false
				for _, a := range attendees {
					if a.ID == youthProfile.ID {
						isAttending = true
						break
					}
				}
				youthVMs = append(youthVMs, profileSignUpVM{
					ProfileID:   youthProfile.ID,
					ProfileName: youthProfile.DisplayName(),
					IsAttending: isAttending,
				})
			}
		}
	}

	sort.Slice(youthVMs, func(i, j int) bool {
		return youthVMs[i].ProfileName < youthVMs[j].ProfileName
	})

	return append(adultVMs, youthVMs...)
}

func (h *EventHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	profileID := r.URL.Query().Get("profile_id")
	if profileID == "" {
		http.Error(w, "Missing profile_id", http.StatusBadRequest)
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

	if !h.canManageProfile(ctx, currentUser.ID, profileID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.repo.SignUp(ctx, eventID, profileID); err != nil {
		log.Printf("SignUp: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	youthVMs, adultVMs := splitAttendeeVMs(attendees)
	profileVMs := h.buildProfileSignUps(ctx, currentUser.ID, attendees)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.ExecuteTemplate(w, "signup_button.html", signupSectionData{
		EventID:  eventID,
		IsPast:   false,
		Profiles: profileVMs,
	}); err != nil {
		log.Printf("template execution (signup_button): %v", err)
	}

	if err := h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
		YouthAttendees: youthVMs,
		YouthCount:     len(youthVMs),
		AdultAttendees: adultVMs,
		AdultCount:     len(adultVMs),
		AttendeeCount:  len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}
}

func (h *EventHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	profileID := r.URL.Query().Get("profile_id")
	if profileID == "" {
		http.Error(w, "Missing profile_id", http.StatusBadRequest)
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

	if !h.canManageProfile(ctx, currentUser.ID, profileID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.repo.Withdraw(ctx, eventID, profileID); err != nil {
		log.Printf("Withdraw: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	youthVMs, adultVMs := splitAttendeeVMs(attendees)
	profileVMs := h.buildProfileSignUps(ctx, currentUser.ID, attendees)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.ExecuteTemplate(w, "signup_button.html", signupSectionData{
		EventID:  eventID,
		IsPast:   false,
		Profiles: profileVMs,
	}); err != nil {
		log.Printf("template execution (signup_button): %v", err)
	}

	if err := h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
		YouthAttendees: youthVMs,
		YouthCount:     len(youthVMs),
		AdultAttendees: adultVMs,
		AdultCount:     len(adultVMs),
		AttendeeCount:  len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}
}

func (h *EventHandler) canManageProfile(ctx context.Context, userID string, profileID string) bool {
	userProfile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return false
	}
	if userProfile.ID == profileID {
		return true
	}
	links, err := h.parentYouthLink.ListByParent(ctx, userProfile.ID)
	if err != nil {
		return false
	}
	for _, link := range links {
		if link.YouthProfileID == profileID && link.Status == parentyouthlink.StatusApproved {
			return true
		}
	}
	return false
}

var muxVars = func(r *http.Request) map[string]string {
	return map[string]string{}
}

func SetMuxVars(fn func(r *http.Request) map[string]string) {
	muxVars = fn
}

func formatCost(cents int) string {
	if cents == 0 {
		return "0.00"
	}
	dollars := cents / 100
	remainder := cents % 100
	return fmt.Sprintf("%d.%02d", dollars, remainder)
}

func splitAttendeeVMs(attendees []*profile.Profile) ([]attendeeViewModel, []attendeeViewModel) {
	var youthVMs []attendeeViewModel
	var adultVMs []attendeeViewModel
	for _, p := range attendees {
		vm := attendeeViewModel{ProfileName: p.DisplayName()}
		if p.MemberType == profile.MemberTypeYouth {
			youthVMs = append(youthVMs, vm)
		} else {
			adultVMs = append(adultVMs, vm)
		}
	}
	sort.Slice(youthVMs, func(i, j int) bool {
		return youthVMs[i].ProfileName < youthVMs[j].ProfileName
	})
	sort.Slice(adultVMs, func(i, j int) bool {
		return adultVMs[i].ProfileName < adultVMs[j].ProfileName
	})
	return youthVMs, adultVMs
}
