package api

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"scout-app/internal/domain/appconfig"
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
	cooking         *event.CookingPatrolService
	tents           *event.TentService
	tmpl            *template.Template
	appConfigRepo   appconfig.Repository
}

type eventsPageData struct {
	Title              string
	IsAdmin            bool
	ProfileID          string
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
	Title              string
	IsAdmin            bool
	ProfileID          string
	Event              *event.Event
	Errors             map[string]string
	FlashSuccess       string
	FormAction         string
	SubmitLabel        string
	StartTimeFormatted string
	EndTimeFormatted   string
	CostFormatted      string
	DescriptionHTML    template.HTML
	UnitType           string
	UnitNumber         string
}

type eventDetailData struct {
	Title           string
	IsAdmin         bool
	ProfileID       string
	Event           *event.Event
	EventID         string
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
	CookingEnabled  bool
	TentingEnabled  bool
	DriversEnabled  bool
}

type attendeeViewModel struct {
	ProfileID        string
	ProfileName      string
	IsDriver         bool
	SeatbeltCount    int
	IsSPL            bool
	IsCoordinator    bool
	IsMedicalOfficer bool
	IsSignedUp       bool
}

type signupSectionData struct {
	EventID  string
	IsPast   bool
	Profiles []profileSignUpVM
}

type attendeeListData struct {
	EventID        string
	IsPast         bool
	IsAdmin        bool
	YouthAttendees []attendeeViewModel
	YouthCount     int
	AdultAttendees []attendeeViewModel
	AdultCount     int
	AttendeeCount  int
}

type cookingSectionData struct {
	EventID          string
	IsPast           bool
	IsAdmin          bool
	Enabled          bool
	YouthPatrols     []cookingPatrolVM
	AdultPatrols     []cookingPatrolVM
	UnassignedYouth  []cookingUnassignedVM
	UnassignedAdults []cookingUnassignedVM
}

type cookingPatrolVM struct {
	ID          string
	Name        string
	IsAdult     bool
	Members     []cookingMemberVM
	MoveTargets []cookingPatrolTargetVM
}

type cookingMemberVM struct {
	ProfileID   string
	ProfileName string
	IsCook      bool
}

type cookingUnassignedVM struct {
	ProfileID   string
	ProfileName string
	IsAdult     bool
	Targets     []cookingPatrolTargetVM
}

type cookingPatrolTargetVM struct {
	ID   string
	Name string
}

type tentingSectionData struct {
	EventID         string
	IsPast          bool
	IsAdmin         bool
	Tents           []tentVM
	UnassignedYouth []tentUnassignedVM
}

type tentVM struct {
	ID         string
	Name       string
	NeedsScout bool
	Alone      bool
	Members    []tentMemberVM
}

type tentMemberVM struct {
	ProfileID   string
	ProfileName string
	Birthdate   string // formatted birthdate (rendered for admins only)
	Age         string // whole years at event start, "—" when birthdate unknown
	MoveTargets []tentTargetVM
}

type tentUnassignedVM struct {
	ProfileID   string
	ProfileName string
	Birthdate   string
	Age         string
	Targets     []tentTargetVM
}

type tentTargetVM struct {
	ID   string
	Name string
}

type eventListPartialData struct {
	Events     []*event.ListItem
	Section    string
	Displayed  int
	Total      int
	NextOffset int
	HasMore    bool
}

func NewEventHandler(repo event.Repository, auth *auth.AuthService, rbac rbac.Repository, profiles profile.Repository, parentYouthLink parentyouthlink.Repository, appConfigRepo appconfig.Repository) *EventHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &EventHandler{
		repo:            repo,
		auth:            auth,
		rbac:            rbac,
		profiles:        profiles,
		parentYouthLink: parentYouthLink,
		cooking:         event.NewCookingPatrolService(repo, profiles),
		tents:           event.NewTentService(repo, profiles, parentYouthLink),
		tmpl:            tmpl,
		appConfigRepo:   appConfigRepo,
	}
}

func (h *EventHandler) loadUnitInfo(ctx context.Context) (unitType, unitNumber string) {
	unitType = appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "UNIT_TYPE", appconfig.KeyUnitType, "Troop")
	unitNumber = appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "UNIT_NUMBER", appconfig.KeyUnitNumber, "")
	return
}

func (h *EventHandler) currentProfileID(r *http.Request) string {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		return ""
	}
	p, err := h.profiles.GetByUserID(r.Context(), user.ID)
	if err != nil || p == nil {
		return ""
	}
	return p.ID
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

	unitType, unitNumber := h.loadUnitInfo(ctx)
	pageTitle := fmt.Sprintf("%s %s Events", unitType, unitNumber)
	data := eventsPageData{
		Title:              pageTitle,
		IsAdmin:            h.isAdmin(ctx, r),
		ProfileID:          h.currentProfileID(r),
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
	evt, err := h.repo.GetByID(ctx, eventID)
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

	isPast := evt.EndTime.Before(time.Now())
	costDisplay := formatCost(evt.CostCents)

	drivers, dErr := h.repo.GetDrivers(ctx, eventID)
	if dErr != nil {
		log.Printf("GetDrivers: %v", dErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	profileVMs := h.buildProfileSignUps(ctx, currentUser.ID, attendees)
	responsibilities, rErr := h.repo.GetResponsibilities(ctx, eventID)
	if rErr != nil {
		log.Printf("GetResponsibilities: %v", rErr)
	}
	youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, responsibilities)

	renderedDesc, err := renderMarkdown(evt.Description)
	if err != nil {
		renderedDesc = template.HTMLEscapeString(evt.Description)
	}

	flashSuccess := ""
	if r.URL.Query().Get("created") == "1" {
		flashSuccess = "Event created successfully!"
	}
	if r.URL.Query().Get("updated") == "1" {
		flashSuccess = "Event updated successfully!"
	}

	unitType, unitNumber := h.loadUnitInfo(ctx)
	detailTitle := fmt.Sprintf("%s %s Events", unitType, unitNumber)

	isAdmin := h.isAdmin(ctx, r)

	data := eventDetailData{
		Title:           detailTitle,
		IsAdmin:         isAdmin,
		ProfileID:       h.currentProfileID(r),
		Event:           evt,
		EventID:         eventID,
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
		CookingEnabled:  evt.CookingEnabled,
		TentingEnabled:  evt.TentingEnabled,
		DriversEnabled:  evt.DriversEnabled,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_detail.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) EventTentingTab(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	eventID := muxVars(r)["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.TentingEnabled {
		http.Error(w, "Tenting not enabled", http.StatusBadRequest)
		return
	}

	data, err := h.buildTentingSectionData(ctx, eventID, evt.StartTime, evt.EndTime.Before(time.Now()), h.isAdmin(ctx, r))
	if err != nil {
		log.Printf("EventTentingTab: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "tent_section.html", data); err != nil {
		log.Printf("template execution (tent_section): %v", err)
	}
}

func (h *EventHandler) EventCookingTab(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	eventID := muxVars(r)["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.CookingEnabled {
		http.Error(w, "Cooking not enabled", http.StatusBadRequest)
		return
	}

	data, err := h.buildCookingSectionData(ctx, eventID, evt.EndTime.Before(time.Now()), h.isAdmin(ctx, r))
	if err != nil {
		log.Printf("EventCookingTab: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.Enabled = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "cooking_section.html", data); err != nil {
		log.Printf("template execution (cooking_section): %v", err)
	}
}

func (h *EventHandler) EventDriversTab(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	eventID := muxVars(r)["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.DriversEnabled {
		http.Error(w, "Drivers not enabled", http.StatusBadRequest)
		return
	}

	userProfile, pErr := h.profiles.GetByUserID(ctx, currentUser.ID)
	if pErr != nil {
		log.Printf("EventDriversTab GetByUserID: %v", pErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, dErr := h.repo.GetDrivers(ctx, eventID)
	if dErr != nil {
		log.Printf("EventDriversTab GetDrivers: %v", dErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	summary, sErr := h.repo.GetSeatbeltSummary(ctx, eventID)
	if sErr != nil {
		log.Printf("EventDriversTab GetSeatbeltSummary: %v", sErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	attendees, aErr := h.repo.GetAttendees(ctx, eventID)
	if aErr != nil {
		log.Printf("EventDriversTab GetAttendees: %v", aErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isDriver, seatbeltCount := computeDriverInfo(userProfile.ID, drivers)
	isSignedUp := isAttending(userProfile.ID, attendees)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
		EventID: eventID, IsPast: evt.EndTime.Before(time.Now()), ProfileID: userProfile.ID,
		IsAdmin: h.isAdmin(ctx, r), IsSignedUp: isSignedUp, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
		Drivers: drivers, Summary: *summary,
	}); err != nil {
		log.Printf("template execution (drivers_section): %v", err)
	}
}

func (h *EventHandler) EventCreateForm(w http.ResponseWriter, r *http.Request) {
	unitType, unitNumber := h.loadUnitInfo(r.Context())
	data := eventFormData{
		Title:       "Create Event",
		IsAdmin:     h.isAdmin(r.Context(), r),
		ProfileID:   h.currentProfileID(r),
		Event:       &event.Event{},
		FormAction:  "/events/create",
		SubmitLabel: "Create Event",
		UnitType:    unitType,
		UnitNumber:  unitNumber,
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

	ctx := r.Context()

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")
	costStr := r.FormValue("cost")
	eventType := r.FormValue("type")
	cookingEnabled := r.FormValue("cooking_enabled") != ""
	tentingEnabled := r.FormValue("tenting_enabled") != ""
	driversEnabled := r.FormValue("drivers_enabled") != ""

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

	evt := &event.Event{
		Title:          title,
		Description:    description,
		Location:       location,
		StartTime:      startTime,
		EndTime:        endTime,
		CostCents:      costCents,
		Type:           eventType,
		CookingEnabled: cookingEnabled,
		TentingEnabled: tentingEnabled,
		DriversEnabled: driversEnabled,
	}

	if len(errors) > 0 {
		errors["cooking"] = "Some fields were invalid. Please fix the indicated errors above."
		data := h.buildFormDataOnError(ctx, r, "Create Event", "/events/create", "Create Event", evt, startTimeStr, endTimeStr, costStr, errors)
		h.renderEventForm(w, r, data, http.StatusUnprocessableEntity)
		return
	}

	evt.CreatedAt = time.Now()
	evt.UpdatedAt = time.Now()

	if err := h.repo.Create(ctx, evt); err != nil {
		log.Printf("EventCreate: %v", err)
		data := h.buildFormDataOnError(ctx, r, "Create Event", "/events/create", "Create Event", evt, startTimeStr, endTimeStr, costStr, map[string]string{"cooking": fmt.Sprintf("Failed to create event. %v", err)})
		h.renderEventForm(w, r, data, http.StatusInternalServerError)
		return
	}

	// Auto-assign Coordinator to event creator
	if creatorProfileID := h.currentProfileID(r); creatorProfileID != "" {
		if err := h.repo.SignUp(ctx, evt.ID, creatorProfileID); err != nil {
			log.Printf("EventCreate SignUp creator: %v", err)
		} else if err := h.repo.AssignResponsibility(ctx, evt.ID, creatorProfileID, event.ResponsibilityCoordinator); err != nil {
			log.Printf("EventCreate AssignCoordinator: %v", err)
		}
		if err := h.cooking.AfterSignUp(ctx, evt.ID, creatorProfileID); err != nil {
			log.Printf("EventCreate AfterSignUp (cooking): %v", err)
		}
	}

	h.redirectToEvent(w, r, evt.ID, "?created=1")
}

func (h *EventHandler) buildEditFormData(ctx context.Context, r *http.Request, evt *event.Event, errors map[string]string) eventFormData {
	unitType, unitNumber := h.loadUnitInfo(ctx)
	costDisplay := fmt.Sprintf("%.2f", float64(evt.CostCents)/100.0)
	renderedDesc, err := renderMarkdown(evt.Description)
	if err != nil {
		renderedDesc = template.HTMLEscapeString(evt.Description)
	}
	return eventFormData{
		Title:              "Edit Event",
		IsAdmin:            true,
		ProfileID:          h.currentProfileID(r),
		Event:              evt,
		Errors:             errors,
		FormAction:         fmt.Sprintf("/events/%s/edit", evt.ID),
		SubmitLabel:        "Save Changes",
		StartTimeFormatted: evt.StartTime.Format("2006-01-02T15:04"),
		EndTimeFormatted:   evt.EndTime.Format("2006-01-02T15:04"),
		CostFormatted:      costDisplay,
		DescriptionHTML:    template.HTML(renderedDesc),
		UnitType:           unitType,
		UnitNumber:         unitNumber,
	}
}

func (h *EventHandler) buildFormDataOnError(ctx context.Context, r *http.Request, title, formAction, submitLabel string, evt *event.Event, startTimeStr, endTimeStr, costStr string, errors map[string]string) eventFormData {
	unitType, unitNumber := h.loadUnitInfo(ctx)
	return eventFormData{
		Title:              title,
		IsAdmin:            true,
		ProfileID:          h.currentProfileID(r),
		Event:              evt,
		Errors:             errors,
		FormAction:         formAction,
		SubmitLabel:        submitLabel,
		StartTimeFormatted: startTimeStr,
		EndTimeFormatted:   endTimeStr,
		CostFormatted:      costStr,
		UnitType:           unitType,
		UnitNumber:         unitNumber,
	}
}

func (h *EventHandler) renderEventForm(w http.ResponseWriter, r *http.Request, data eventFormData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	tmplName := "event_form.html"
	if r.Header.Get("HX-Request") != "" {
		tmplName = "event_form_content.html"
	}
	if err := h.tmpl.ExecuteTemplate(w, tmplName, data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) redirectToEvent(w http.ResponseWriter, r *http.Request, eventID string, query string) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/events/%s%s", eventID, query))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/events/%s%s", eventID, query), http.StatusFound)
}

func (h *EventHandler) EventEditForm(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("EventEditForm GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	data := h.buildEditFormData(ctx, r, evt, nil)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_form.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) EventEdit(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	existing, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("EventEdit GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")
	costStr := r.FormValue("cost")
	eventType := r.FormValue("type")
	cookingEnabled := r.FormValue("cooking_enabled") != ""
	tentingEnabled := r.FormValue("tenting_enabled") != ""
	driversEnabled := r.FormValue("drivers_enabled") != ""

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
		errors["cooking"] = "Some fields were invalid. Please fix the indicated errors above."
		evt := &event.Event{
			ID:          eventID,
			Title:       title,
			Description: description,
			Location:    location,
			StartTime:   startTime,
			EndTime:     endTime,
			CostCents:   costCents,
			Type:        eventType,
			CreatedAt:   existing.CreatedAt,
		}
		data := h.buildEditFormData(ctx, r, evt, errors)
		h.renderEventForm(w, r, data, http.StatusUnprocessableEntity)
		return
	}

	evt := &event.Event{
		ID:             eventID,
		Title:          title,
		Description:    description,
		Location:       location,
		StartTime:      startTime,
		EndTime:        endTime,
		CostCents:      costCents,
		Type:           eventType,
		CookingEnabled: cookingEnabled,
		TentingEnabled: tentingEnabled,
		DriversEnabled: driversEnabled,
		CreatedAt:      existing.CreatedAt,
	}

	if err := h.repo.UpdateWithCooking(ctx, evt, existing.CookingEnabled); err != nil {
		log.Printf("EventEdit UpdateWithCooking: %v", err)
		data := h.buildEditFormData(ctx, r, evt, map[string]string{"cooking": fmt.Sprintf("Failed to update event. %v", err)})
		h.renderEventForm(w, r, data, http.StatusInternalServerError)
		return
	}

	h.redirectToEvent(w, r, eventID, "?updated=1")
}

func (h *EventHandler) buildProfileSignUps(ctx context.Context, currentUserID string, attendees []*profile.Profile) []profileSignUpVM {
	var youthVMs []profileSignUpVM
	var adultVMs []profileSignUpVM

	userProfile, err := h.profiles.GetByUserID(ctx, currentUserID)
	if err == nil && userProfile.Status == profile.StatusActive {
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

	if userProfile != nil && userProfile.Status == profile.StatusActive {
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
				if youthProfile.Status != profile.StatusActive {
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

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot sign up for a past event", http.StatusBadRequest)
		return
	}

	if !h.canManageProfile(ctx, currentUser.ID, profileID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	profileToSignUp, err := h.profiles.GetByID(ctx, profileID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if profileToSignUp.Status == profile.StatusInactive {
		http.Error(w, "Cannot sign up an inactive profile", http.StatusBadRequest)
		return
	}

	if err := h.repo.SignUp(ctx, eventID, profileID); err != nil {
		log.Printf("SignUp: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-assign SPL for youth with Senior Patrol Leader position
	if profileToSignUp.MemberType == profile.MemberTypeYouth && strings.Contains(strings.ToLower(profileToSignUp.Positions), strings.ToLower("Senior Patrol Leader")) {
		if err := h.repo.AssignResponsibility(ctx, eventID, profileID, event.ResponsibilitySPL); err != nil {
			log.Printf("auto-assign SPL: %v", err)
		}
	}

	// Auto-join adults into the Adult Cooking Patrol when cooking is enabled
	if err := h.cooking.AfterSignUp(ctx, eventID, profileID); err != nil {
		log.Printf("AfterSignUp (cooking): %v", err)
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
	if err != nil {
		log.Printf("GetSeatbeltSummary: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	responsibilities, _ := h.repo.GetResponsibilities(ctx, eventID)
	youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, responsibilities)
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
		EventID:        eventID,
		IsPast:         false,
		IsAdmin:        h.isAdmin(ctx, r),
		YouthAttendees: youthVMs,
		YouthCount:     len(youthVMs),
		AdultAttendees: adultVMs,
		AdultCount:     len(adultVMs),
		AttendeeCount:  len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}

	if evt.CookingEnabled {
		h.renderCookingSection(w, r, eventID)
	}

	if evt.TentingEnabled {
		h.renderTentingSection(w, r, evt)
	}

	// Show driver sign-up option for adult self-signups
	if evt.DriversEnabled && profileToSignUp.MemberType == profile.MemberTypeAdult {
		isDriver, seatbeltCount := computeDriverInfo(profileToSignUp.ID, drivers)
		if err := h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
			EventID: eventID, IsPast: false, ProfileID: profileToSignUp.ID,
			IsAdmin: h.isAdmin(ctx, r), IsSignedUp: true, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
			Drivers: drivers, Summary: *summary,
		}); err != nil {
			log.Printf("template execution (drivers_section): %v", err)
		}
		if !isDriver {
			if err := h.tmpl.ExecuteTemplate(w, "driver_modal.html", driversSectionData{
				EventID: eventID, IsPast: false, ProfileID: profileToSignUp.ID,
				IsAdmin: h.isAdmin(ctx, r), IsSignedUp: true, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
				Drivers: drivers, Summary: *summary,
			}); err != nil {
				log.Printf("template execution (driver_modal): %v", err)
			}
		}
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

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot withdraw from a past event", http.StatusBadRequest)
		return
	}

	profileToWithdraw, err := h.profiles.GetByID(ctx, profileID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if profileToWithdraw.Status == profile.StatusInactive && !h.isAdmin(ctx, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !h.canManageProfile(ctx, currentUser.ID, profileID) && !h.isAdmin(ctx, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.repo.Withdraw(ctx, eventID, profileID); err != nil {
		log.Printf("Withdraw: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Cascade: also remove driver responsibility if present
	_ = h.repo.RemoveDriver(ctx, eventID, profileID)
	_ = h.repo.RemoveResponsibility(ctx, eventID, profileID, event.ResponsibilitySPL)
	_ = h.repo.RemoveResponsibility(ctx, eventID, profileID, event.ResponsibilityCoordinator)
	_ = h.repo.RemoveResponsibility(ctx, eventID, profileID, event.ResponsibilityMedicalOfficer)
	_ = h.repo.RemoveResponsibility(ctx, eventID, profileID, event.ResponsibilityDriver)

	// Cascade: remove from cooking patrol and clear cook designation if held
	if err := h.cooking.AfterWithdraw(ctx, eventID, profileID); err != nil {
		log.Printf("AfterWithdraw (cooking): %v", err)
	}

	// Cascade: remove from tent if the attendee was assigned to one
	if err := h.tents.AfterWithdraw(ctx, eventID, profileID); err != nil {
		log.Printf("AfterWithdraw (tenting): %v", err)
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	withdrawResponsibilities, _ := h.repo.GetResponsibilities(ctx, eventID)
	youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, withdrawResponsibilities)
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
		EventID:        eventID,
		IsPast:         false,
		IsAdmin:        h.isAdmin(ctx, r),
		YouthAttendees: youthVMs,
		YouthCount:     len(youthVMs),
		AdultAttendees: adultVMs,
		AdultCount:     len(adultVMs),
		AttendeeCount:  len(attendees),
	}); err != nil {
		log.Printf("template execution (attendee_list): %v", err)
	}

	if evt.CookingEnabled {
		h.renderCookingSection(w, r, eventID)
	}

	if evt.TentingEnabled {
		h.renderTentingSection(w, r, evt)
	}

	if evt.DriversEnabled {
		summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
		if err != nil {
			log.Printf("GetSeatbeltSummary: %v", err)
			return
		}

		userProfile, err := h.profiles.GetByUserID(ctx, currentUser.ID)
		if err != nil {
			log.Printf("GetByUserID: %v", err)
			return
		}

		isDriver, seatbeltCount := computeDriverInfo(userProfile.ID, drivers)
		isSignedUp := isAttending(userProfile.ID, attendees)
		if err := h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
			EventID: eventID, IsPast: false, ProfileID: userProfile.ID,
			IsAdmin: h.isAdmin(ctx, r), IsSignedUp: isSignedUp, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
			Drivers: drivers, Summary: *summary,
		}); err != nil {
			log.Printf("template execution (drivers_section): %v", err)
		}
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

type eventDeleteData struct {
	EventID string
	Title   string
	Error   string
}

func (h *EventHandler) EventDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	evt, err := h.repo.GetByID(r.Context(), eventID)
	if err != nil {
		log.Printf("EventDeleteConfirm GetByID: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	data := eventDeleteData{
		EventID: evt.ID,
		Title:   evt.Title,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "event_delete_confirm.html", data); err != nil {
		log.Printf("template execution: %v", err)
	}
}

func (h *EventHandler) EventDelete(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), eventID); err != nil {
		log.Printf("EventDelete: %v", err)
		data := eventDeleteData{
			EventID: eventID,
			Error:   "Failed to delete event",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := h.tmpl.ExecuteTemplate(w, "event_delete_confirm.html", data); err != nil {
			log.Printf("template execution: %v", err)
		}
		return
	}

	w.Header().Set("HX-Redirect", "/events?deleted=1")
	w.WriteHeader(http.StatusOK)
}

type driversSectionData struct {
	EventID       string
	IsPast        bool
	ProfileID     string
	IsAdmin       bool
	IsSignedUp    bool
	IsDriver      bool
	SeatbeltCount int
	Drivers       []event.DriverResponsibility
	Summary       event.SeatbeltSummary
}

func computeDriverInfo(profileID string, drivers []event.DriverResponsibility) (bool, int) {
	for _, d := range drivers {
		if d.ProfileID == profileID {
			return true, d.SeatbeltCount
		}
	}
	return false, 0
}

func isAttending(profileID string, attendees []*profile.Profile) bool {
	for _, a := range attendees {
		if a.ID == profileID {
			return true
		}
	}
	return false
}

func (h *EventHandler) AddDriver(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	seatbeltCountStr := r.FormValue("seatbelt_count")
	if seatbeltCountStr == "" {
		http.Error(w, "Missing seatbelt_count", http.StatusBadRequest)
		return
	}

	seatbeltCount, err := strconv.Atoi(seatbeltCountStr)
	if err != nil || seatbeltCount < 1 {
		http.Error(w, "Invalid seatbelt_count", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.DriversEnabled {
		http.Error(w, "Drivers not enabled", http.StatusBadRequest)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot modify drivers for a past event", http.StatusBadRequest)
		return
	}

	userProfile, err := h.profiles.GetByUserID(ctx, currentUser.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.repo.AddDriver(ctx, eventID, userProfile.ID, seatbeltCount); err != nil {
		log.Printf("AddDriver: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
	if err != nil {
		log.Printf("GetSeatbeltSummary: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isDriver, seatbeltCount := computeDriverInfo(userProfile.ID, drivers)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	attendees, aErr := h.repo.GetAttendees(ctx, eventID)
	if aErr == nil {
		responsibilities, _ := h.repo.GetResponsibilities(ctx, eventID)
		youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, responsibilities)
		h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
			EventID: eventID, IsPast: false, IsAdmin: h.isAdmin(ctx, r),
			YouthAttendees: youthVMs, YouthCount: len(youthVMs),
			AdultAttendees: adultVMs, AdultCount: len(adultVMs),
			AttendeeCount: len(attendees),
		})
	}

	h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
		EventID: eventID, IsPast: false, ProfileID: userProfile.ID,
		IsAdmin: h.isAdmin(ctx, r), IsSignedUp: true, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
		Drivers: drivers, Summary: *summary,
	})
	w.Write([]byte(`<div id="modal-container" hx-swap-oob="delete"></div>`))
}

func (h *EventHandler) RemoveDriver(w http.ResponseWriter, r *http.Request) {
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

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.DriversEnabled {
		http.Error(w, "Drivers not enabled", http.StatusBadRequest)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot modify drivers for a past event", http.StatusBadRequest)
		return
	}

	userProfile, err := h.profiles.GetByUserID(ctx, currentUser.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.repo.RemoveDriver(ctx, eventID, userProfile.ID); err != nil {
		log.Printf("RemoveDriver: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
	if err != nil {
		log.Printf("GetSeatbeltSummary: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isDriver, seatbeltCount := computeDriverInfo(userProfile.ID, drivers)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	attendees, aErr := h.repo.GetAttendees(ctx, eventID)
	if aErr == nil {
		responsibilities, _ := h.repo.GetResponsibilities(ctx, eventID)
		youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, responsibilities)
		h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
			EventID: eventID, IsPast: false, IsAdmin: h.isAdmin(ctx, r),
			YouthAttendees: youthVMs, YouthCount: len(youthVMs),
			AdultAttendees: adultVMs, AdultCount: len(adultVMs),
			AttendeeCount: len(attendees),
		})
	}

	h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
		EventID: eventID, IsPast: false, ProfileID: userProfile.ID,
		IsAdmin: h.isAdmin(ctx, r), IsSignedUp: true, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
		Drivers: drivers, Summary: *summary,
	})
}

func (h *EventHandler) UpdateDriverSeatbelt(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	seatbeltCountStr := r.FormValue("seatbelt_count")
	if seatbeltCountStr == "" {
		http.Error(w, "Missing seatbelt_count", http.StatusBadRequest)
		return
	}

	seatbeltCount, err := strconv.Atoi(seatbeltCountStr)
	if err != nil || seatbeltCount < 1 {
		http.Error(w, "Invalid seatbelt_count", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if !evt.DriversEnabled {
		http.Error(w, "Drivers not enabled", http.StatusBadRequest)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot modify drivers for a past event", http.StatusBadRequest)
		return
	}

	userProfile, err := h.profiles.GetByUserID(ctx, currentUser.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.repo.UpdateDriverSeatbeltCount(ctx, eventID, userProfile.ID, seatbeltCount); err != nil {
		log.Printf("UpdateDriverSeatbeltCount: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
	if err != nil {
		log.Printf("GetSeatbeltSummary: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isDriver, seatbeltCount := computeDriverInfo(userProfile.ID, drivers)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	attendees, aErr := h.repo.GetAttendees(ctx, eventID)
	if aErr == nil {
		responsibilities, _ := h.repo.GetResponsibilities(ctx, eventID)
		youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, responsibilities)
		h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
			EventID: eventID, IsPast: false, IsAdmin: h.isAdmin(ctx, r),
			YouthAttendees: youthVMs, YouthCount: len(youthVMs),
			AdultAttendees: adultVMs, AdultCount: len(adultVMs),
			AttendeeCount: len(attendees),
		})
	}

	h.tmpl.ExecuteTemplate(w, "drivers_section.html", driversSectionData{
		EventID: eventID, IsPast: false, ProfileID: userProfile.ID,
		IsAdmin: h.isAdmin(ctx, r), IsSignedUp: true, IsDriver: isDriver, SeatbeltCount: seatbeltCount,
		Drivers: drivers, Summary: *summary,
	})
}

func (h *EventHandler) ToggleResponsibility(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	eventID := vars["id"]
	profileID := vars["profile_id"]
	respParam := vars["responsibility"]
	if eventID == "" || profileID == "" || respParam == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	respType := event.Responsibility(respParam)
	if respType == event.ResponsibilityDriver {
		http.Error(w, "Driver responsibility is managed separately", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}
	if evt.EndTime.Before(time.Now()) {
		http.Error(w, "Cannot modify responsibilities for a past event", http.StatusBadRequest)
		return
	}

	if !h.canManageProfile(ctx, currentUser.ID, profileID) && !h.isAdmin(ctx, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	attendees, aErr := h.repo.GetAttendees(ctx, eventID)
	if aErr != nil {
		log.Printf("GetAttendees: %v", aErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isAttending(profileID, attendees) {
		http.Error(w, "Profile is not signed up", http.StatusBadRequest)
		return
	}

	drivers, dErr := h.repo.GetDrivers(ctx, eventID)
	if dErr != nil {
		log.Printf("GetDrivers: %v", dErr)
	}

	currentResp, rErr := h.repo.GetResponsibilities(ctx, eventID)
	if rErr != nil {
		log.Printf("GetResponsibilities: %v", rErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	alreadyAssigned := false
	for _, ra := range currentResp {
		if ra.ProfileID == profileID && ra.Responsibility == respType {
			alreadyAssigned = true
			break
		}
	}

	if alreadyAssigned {
		if err := h.repo.RemoveResponsibility(ctx, eventID, profileID, respType); err != nil {
			log.Printf("RemoveResponsibility: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.repo.AssignResponsibility(ctx, eventID, profileID, respType); err != nil {
			log.Printf("AssignResponsibility: %v", err)
			if sec, ok := err.(event.ErrSingletonConflict); ok {
				if err := h.repo.RemoveResponsibility(ctx, eventID, sec.CurrentHolderID, respType); err != nil {
					log.Printf("RemoveResponsibility (replace): %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				if err := h.repo.AssignResponsibility(ctx, eventID, profileID, respType); err != nil {
					log.Printf("AssignResponsibility (replace): %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}
	}

	updatedResp, _ := h.repo.GetResponsibilities(ctx, eventID)
	youthVMs, adultVMs := splitAttendeeVMs(attendees, drivers, updatedResp)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "attendee_list.html", attendeeListData{
		EventID: eventID, IsPast: false, IsAdmin: h.isAdmin(ctx, r),
		YouthAttendees: youthVMs, YouthCount: len(youthVMs),
		AdultAttendees: adultVMs, AdultCount: len(adultVMs),
		AttendeeCount: len(attendees),
	})
}

func (h *EventHandler) cookingPatrolPrereqs(r *http.Request) (string, int, string) {
	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		return "", http.StatusUnauthorized, "Unauthorized"
	}
	if !h.isAdmin(ctx, r) {
		return "", http.StatusForbidden, "Forbidden"
	}
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		return "", http.StatusBadRequest, "Bad request"
	}
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		return "", http.StatusNotFound, "Event not found"
	}
	if !evt.CookingEnabled {
		return "", http.StatusBadRequest, "Cooking not enabled"
	}
	if evt.EndTime.Before(time.Now()) {
		return "", http.StatusBadRequest, "Cannot modify cooking patrols for a past event"
	}
	return eventID, 0, ""
}

func (h *EventHandler) renderCookingSection(w http.ResponseWriter, r *http.Request, eventID string) {
	ctx := r.Context()
	data, err := h.buildCookingSectionData(ctx, eventID, false, h.isAdmin(ctx, r))
	if err != nil {
		log.Printf("renderCookingSection: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data.Enabled = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "cooking_section.html", data); err != nil {
		log.Printf("template execution (cooking_section): %v", err)
	}
}

func (h *EventHandler) CookingCreatePatrol(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	if _, err := h.cooking.CreatePatrol(r.Context(), eventID, false); err != nil {
		log.Printf("CreatePatrol: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
}

func (h *EventHandler) CookingDeletePatrol(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	patrolID := muxVars(r)["patrol_id"]
	if patrolID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.cooking.DeletePatrol(r.Context(), patrolID); err != nil {
		log.Printf("DeletePatrol: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
}

func (h *EventHandler) CookingAssignMember(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	profileID := r.FormValue("profile_id")
	patrolID := r.FormValue("patrol_id")
	if profileID == "" || patrolID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.cooking.AssignMember(r.Context(), eventID, patrolID, profileID); err != nil {
		if errors.Is(err, event.ErrAdultInYouthPatrol) {
			http.Error(w, "Adults cannot be placed into a youth cooking patrol", http.StatusBadRequest)
			return
		}
		log.Printf("AssignMember: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
}

func (h *EventHandler) CookingRemoveMember(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	vars := muxVars(r)
	patrolID := vars["patrol_id"]
	profileID := vars["profile_id"]
	if patrolID == "" || profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.cooking.RemoveMember(r.Context(), eventID, profileID); err != nil {
		log.Printf("RemoveMember: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
}

func (h *EventHandler) CookingSetCook(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	ctx := r.Context()
	vars := muxVars(r)
	patrolID := vars["patrol_id"]
	profileID := vars["profile_id"]
	if patrolID == "" || profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	patrols, err := h.cooking.ListPatrols(ctx, eventID)
	if err != nil {
		log.Printf("ListPatrols: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	var patrolFound bool
	for _, p := range patrols {
		if p.ID == patrolID {
			patrolFound = true
			break
		}
	}
	if !patrolFound {
		http.Error(w, "Patrol not found", http.StatusNotFound)
		return
	}

	if err := h.cooking.SetCook(ctx, eventID, patrolID, profileID); err != nil {
		log.Printf("SetCook: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
}

func (h *EventHandler) CookingClearCook(w http.ResponseWriter, r *http.Request) {
	eventID, status, msg := h.cookingPatrolPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	patrolID := muxVars(r)["patrol_id"]
	if patrolID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.cooking.ClearCook(r.Context(), eventID, patrolID); err != nil {
		log.Printf("ClearCook: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderCookingSection(w, r, eventID)
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

func enrichVMsWithResponsibilities(vms []attendeeViewModel, responsibilities []event.ResponsibilityAssignment) {
	for i := range vms {
		for _, r := range responsibilities {
			if vms[i].ProfileID == r.ProfileID {
				switch r.Responsibility {
				case event.ResponsibilitySPL:
					vms[i].IsSPL = true
				case event.ResponsibilityCoordinator:
					vms[i].IsCoordinator = true
				case event.ResponsibilityMedicalOfficer:
					vms[i].IsMedicalOfficer = true
				}
			}
		}
	}
}

func enrichVMsWithDrivers(vms []attendeeViewModel, drivers []event.DriverResponsibility) {
	for i := range vms {
		for _, d := range drivers {
			if vms[i].ProfileID == d.ProfileID {
				vms[i].IsDriver = true
				vms[i].SeatbeltCount = d.SeatbeltCount
				break
			}
		}
	}
}

func splitAttendeeVMs(attendees []*profile.Profile, drivers []event.DriverResponsibility, responsibilities []event.ResponsibilityAssignment) ([]attendeeViewModel, []attendeeViewModel) {
	var youthVMs []attendeeViewModel
	var adultVMs []attendeeViewModel
	for _, p := range attendees {
		vm := attendeeViewModel{
			ProfileID:   p.ID,
			ProfileName: p.DisplayName(),
			IsSignedUp:  true,
		}
		if p.MemberType == profile.MemberTypeYouth {
			youthVMs = append(youthVMs, vm)
		} else {
			adultVMs = append(adultVMs, vm)
		}
	}
	enrichVMsWithDrivers(youthVMs, drivers)
	enrichVMsWithDrivers(adultVMs, drivers)
	enrichVMsWithResponsibilities(youthVMs, responsibilities)
	enrichVMsWithResponsibilities(adultVMs, responsibilities)
	sort.Slice(youthVMs, func(i, j int) bool {
		return youthVMs[i].ProfileName < youthVMs[j].ProfileName
	})
	sort.Slice(adultVMs, func(i, j int) bool {
		return adultVMs[i].ProfileName < adultVMs[j].ProfileName
	})
	return youthVMs, adultVMs
}

func (h *EventHandler) buildCookingSectionData(ctx context.Context, eventID string, isPast, isAdmin bool) (*cookingSectionData, error) {
	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		return nil, err
	}
	patrols, err := h.cooking.ListPatrols(ctx, eventID)
	if err != nil {
		return nil, err
	}

	data := &cookingSectionData{
		EventID: eventID,
		IsPast:  isPast,
		IsAdmin: isAdmin,
	}

	memberID := map[string]bool{}
	for _, p := range patrols {
		vm := cookingPatrolVM{ID: p.ID, Name: p.Name, IsAdult: p.IsAdult}
		for _, m := range p.Members {
			memberID[m.ProfileID] = true
			vm.Members = append(vm.Members, cookingMemberVM{
				ProfileID:   m.ProfileID,
				ProfileName: m.ProfileName,
				IsCook:      m.IsCook,
			})
		}
		vm.MoveTargets = cookingTargets(patrols, p.ID, p.IsAdult)
		if p.IsAdult {
			data.AdultPatrols = append(data.AdultPatrols, vm)
		} else {
			data.YouthPatrols = append(data.YouthPatrols, vm)
		}
	}

	for _, a := range attendees {
		if memberID[a.ID] {
			continue
		}
		isAdult := a.MemberType == profile.MemberTypeAdult
		vm := cookingUnassignedVM{
			ProfileID:   a.ID,
			ProfileName: a.DisplayName(),
			IsAdult:     isAdult,
			Targets:     cookingTargets(patrols, "", isAdult),
		}
		if isAdult {
			data.UnassignedAdults = append(data.UnassignedAdults, vm)
		} else {
			data.UnassignedYouth = append(data.UnassignedYouth, vm)
		}
	}

	for _, list := range [][]cookingPatrolVM{data.YouthPatrols, data.AdultPatrols} {
		for i := range list {
			sort.Slice(list[i].Members, func(j, k int) bool {
				return list[i].Members[j].ProfileName < list[i].Members[k].ProfileName
			})
		}
	}
	sort.Slice(data.UnassignedYouth, func(i, j int) bool {
		return data.UnassignedYouth[i].ProfileName < data.UnassignedYouth[j].ProfileName
	})
	sort.Slice(data.UnassignedAdults, func(i, j int) bool {
		return data.UnassignedAdults[i].ProfileName < data.UnassignedAdults[j].ProfileName
	})

	return data, nil
}

func cookingTargets(patrols []*event.CookingPatrol, excludeID string, isAdult bool) []cookingPatrolTargetVM {
	var targets []cookingPatrolTargetVM
	for _, p := range patrols {
		if p.ID == excludeID {
			continue
		}
		if isAdult && !p.IsAdult {
			continue
		}
		targets = append(targets, cookingPatrolTargetVM{ID: p.ID, Name: p.Name})
	}
	return targets
}

func (h *EventHandler) tentPrereqs(r *http.Request) (*event.Event, int, string) {
	ctx := r.Context()
	currentUser, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || currentUser == nil {
		return nil, http.StatusUnauthorized, "Unauthorized"
	}
	if !h.isAdmin(ctx, r) {
		return nil, http.StatusForbidden, "Forbidden"
	}
	vars := muxVars(r)
	eventID := vars["id"]
	if eventID == "" {
		return nil, http.StatusBadRequest, "Bad request"
	}
	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, http.StatusNotFound, "Event not found"
	}
	if !evt.TentingEnabled {
		return nil, http.StatusBadRequest, "Tenting not enabled"
	}
	if evt.EndTime.Before(time.Now()) {
		return nil, http.StatusBadRequest, "Cannot modify tents for a past event"
	}
	return evt, 0, ""
}

func (h *EventHandler) tentMaxAgeGap(ctx context.Context) int {
	v := appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "MAX_TENT_AGE_GAP", appconfig.KeyMaxTentAgeGap, "2")
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 2
	}
	return n
}

func (h *EventHandler) renderTentingSection(w http.ResponseWriter, r *http.Request, evt *event.Event) {
	ctx := r.Context()
	data, err := h.buildTentingSectionData(ctx, evt.ID, evt.StartTime, false, h.isAdmin(ctx, r))
	if err != nil {
		log.Printf("renderTentingSection: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "tent_section.html", data); err != nil {
		log.Printf("template execution (tent_section): %v", err)
	}
}

func (h *EventHandler) buildTentingSectionData(ctx context.Context, eventID string, startTime time.Time, isPast, isAdmin bool) (*tentingSectionData, error) {
	tents, err := h.tents.ListTents(ctx, eventID)
	if err != nil {
		return nil, err
	}
	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*profile.Profile, len(attendees))
	for _, a := range attendees {
		byID[a.ID] = a
	}

	data := &tentingSectionData{
		EventID: eventID,
		IsPast:  isPast,
		IsAdmin: isAdmin,
	}

	memberID := map[string]bool{}
	maxAgeGap := h.tentMaxAgeGap(ctx)
	for _, t := range tents {
		vm := tentVM{ID: t.ID, Name: t.Name, NeedsScout: len(t.Members) < 2, Alone: len(t.Members) == 1}
		for _, m := range t.Members {
			memberID[m.ProfileID] = true
			valid, err := h.tents.ValidTargetTents(ctx, eventID, m.ProfileID, t.ID, maxAgeGap)
			if err != nil {
				return nil, err
			}
			mm := tentMemberVM{
				ProfileID:   m.ProfileID,
				ProfileName: m.ProfileName,
				Birthdate:   "—",
				Age:         "—",
				MoveTargets: tentTargetVMs(valid),
			}
			if p := byID[m.ProfileID]; p != nil {
				mm.Age = tentAgeLabel(p.Birthdate, startTime)
				mm.Birthdate = tentBirthdateLabel(p.Birthdate)
			}
			vm.Members = append(vm.Members, mm)
		}
		sort.Slice(vm.Members, func(i, j int) bool {
			return vm.Members[i].ProfileName < vm.Members[j].ProfileName
		})
		data.Tents = append(data.Tents, vm)
	}

	for _, a := range attendees {
		if a.MemberType != profile.MemberTypeYouth {
			continue
		}
		if memberID[a.ID] {
			continue
		}
		valid, err := h.tents.ValidTargetTents(ctx, eventID, a.ID, "", maxAgeGap)
		if err != nil {
			return nil, err
		}
		data.UnassignedYouth = append(data.UnassignedYouth, tentUnassignedVM{
			ProfileID:   a.ID,
			ProfileName: a.DisplayName(),
			Birthdate:   tentBirthdateLabel(a.Birthdate),
			Age:         tentAgeLabel(a.Birthdate, startTime),
			Targets:     tentTargetVMs(valid),
		})
	}
	sort.Slice(data.UnassignedYouth, func(i, j int) bool {
		return data.UnassignedYouth[i].ProfileName < data.UnassignedYouth[j].ProfileName
	})
	return data, nil
}

// missingBirthdate reports whether a profile birthdate is unknown: the zero
// time or the pre-/1970 sentinels (Go zero value or the schema default) that
// can only occur when Scoutbook never supplied a date. A year-1970 birthdate
// is implausible for any current youth.
func missingBirthdate(bd time.Time) bool {
	return bd.IsZero() || bd.Year() <= 1970
}

// tentBirthdateLabel renders a profile's birthdate for the tenting UI. Unknown
// birthdates are shown as an em dash so they are visibly missing.
func tentBirthdateLabel(bd time.Time) string {
	if missingBirthdate(bd) {
		return "—"
	}
	return bd.Format("Jan 2, 2006")
}

// tentAgeLabel renders a scout's whole-year age on the event start date for
// the tenting UI, matching the rule engine's AgeWholeYears computation. Unknown
// birthdates display as an em dash.
func tentAgeLabel(bd time.Time, at time.Time) string {
	if missingBirthdate(bd) {
		return "—"
	}
	return strconv.Itoa(event.AgeWholeYears(bd, at))
}

func tentTargetVMs(tents []*event.Tent) []tentTargetVM {
	vms := make([]tentTargetVM, 0, len(tents))
	for _, t := range tents {
		vms = append(vms, tentTargetVM{ID: t.ID, Name: t.Name})
	}
	return vms
}

func (h *EventHandler) TentCreate(w http.ResponseWriter, r *http.Request) {
	evt, status, msg := h.tentPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	if _, err := h.tents.CreateTent(r.Context(), evt.ID); err != nil {
		log.Printf("CreateTent: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderTentingSection(w, r, evt)
}

func (h *EventHandler) TentDelete(w http.ResponseWriter, r *http.Request) {
	evt, status, msg := h.tentPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	tentID := muxVars(r)["tent_id"]
	if tentID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.tents.DeleteTent(r.Context(), tentID); err != nil {
		log.Printf("DeleteTent: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderTentingSection(w, r, evt)
}

func (h *EventHandler) TentAssignMember(w http.ResponseWriter, r *http.Request) {
	evt, status, msg := h.tentPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	profileID := r.FormValue("profile_id")
	tentID := r.FormValue("tent_id")
	if profileID == "" || tentID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	violations, err := h.tents.AssignMember(ctx, evt.ID, tentID, profileID, h.tentMaxAgeGap(ctx))
	if err != nil {
		if errors.Is(err, event.ErrAdultInTent) {
			http.Error(w, "Adults cannot be placed into a tent", http.StatusBadRequest)
			return
		}
		log.Printf("AssignMember: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(violations) > 0 {
		messages := make([]string, len(violations))
		for i, v := range violations {
			messages[i] = v.Message
		}
		http.Error(w, strings.Join(messages, "; "), http.StatusBadRequest)
		return
	}

	h.renderTentingSection(w, r, evt)
}

func (h *EventHandler) TentRemoveMember(w http.ResponseWriter, r *http.Request) {
	evt, status, msg := h.tentPrereqs(r)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	profileID := muxVars(r)["profile_id"]
	if profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := h.tents.RemoveMember(r.Context(), evt.ID, profileID); err != nil {
		log.Printf("RemoveMember: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.renderTentingSection(w, r, evt)
}
