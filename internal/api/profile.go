package api

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

type ProfileHandler struct {
	profileRepo     profile.Repository
	eventRepo       event.Repository
	auth            *auth.AuthService
	rbac            rbac.Repository
	parentYouthLink parentyouthlink.Repository
	tmpl            *template.Template
}

type profilePageData struct {
	Title       string
	IsAdmin     bool
	ProfileID   string
	Profile     *profile.Profile
	IsSelf      bool
	IsConnected bool
	ViewerIsAdmin bool
	ShowEmail   bool
	ShowBSAID   bool
	ShowPhone   bool
	ShowBirthdate bool
	IsRegistered bool
	CanGrantAdmin bool
	IsAdminCurrently bool
}

type profileEventListData struct {
	ProfileID string
	Events    []*event.ListItem
	Section   string
	Displayed int
	Total     int
	NextOffset int
	HasMore   bool
}

func NewProfileHandler(
	profileRepo profile.Repository,
	eventRepo event.Repository,
	auth *auth.AuthService,
	rbac rbac.Repository,
	parentYouthLink parentyouthlink.Repository,
) *ProfileHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &ProfileHandler{
		profileRepo:     profileRepo,
		eventRepo:       eventRepo,
		auth:            auth,
		rbac:           rbac,
		parentYouthLink: parentYouthLink,
		tmpl:            tmpl,
	}
}

func (h *ProfileHandler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	vars := muxVars(r)
	profileID := vars["id"]
	if profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	p, err := h.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		log.Printf("ProfilePage GetByID: %v", err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	viewerProfile, err := h.profileRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		viewerProfile = nil
	}

	isSelf := viewerProfile != nil && viewerProfile.ID == profileID

	isConnected := false
	if viewerProfile != nil && !isSelf {
		links, err := h.parentYouthLink.ListByParent(ctx, viewerProfile.ID)
		if err == nil {
			for _, link := range links {
				if link.YouthProfileID == profileID && link.Status == parentyouthlink.StatusApproved {
					isConnected = true
					break
				}
			}
		}
	}

	hasAdminRBAC := false
	perms, err := h.rbac.GetUserPermissions(ctx, user.ID)
	if err == nil {
		for _, perm := range perms {
			if perm.Name == "admin:rbac" {
				hasAdminRBAC = true
				break
			}
		}
	}

	viewerIsAdmin := hasAdminRBAC

	showEmail := isSelf || viewerIsAdmin || isConnected
	showBSAID := isSelf || viewerIsAdmin || isConnected
	showPhone := isSelf || viewerIsAdmin || isConnected
	showBirthdate := true
	if p.MemberType == profile.MemberTypeAdult && !isSelf && !viewerIsAdmin && !isConnected {
		showBirthdate = false
	}

	isRegistered := p.UserID != nil && *p.UserID != ""

	canGrantAdmin := viewerIsAdmin && p.MemberType == profile.MemberTypeAdult && isRegistered

	isAdminCurrently := false
	if isRegistered {
		adminRole, err := h.rbac.GetRoleByName(ctx, "admin")
		if err == nil {
			roles, err := h.rbac.GetUserRoles(ctx, *p.UserID)
			if err == nil {
				for _, role := range roles {
					if role.ID == adminRole.ID {
						isAdminCurrently = true
						break
					}
				}
			}
		}
	}

	hasEventPerm := false
	for _, perm := range perms {
		if perm.Name == "event:create" {
			hasEventPerm = true
			break
		}
	}

	data := profilePageData{
		Title:             p.DisplayName(),
		IsAdmin:           hasEventPerm,
		ProfileID:         viewerProfile.ID,
		Profile:           p,
		IsSelf:            isSelf,
		IsConnected:       isConnected,
		ViewerIsAdmin:     viewerIsAdmin,
		ShowEmail:         showEmail,
		ShowBSAID:         showBSAID,
		ShowPhone:         showPhone,
		ShowBirthdate:     showBirthdate,
		IsRegistered:      isRegistered,
		CanGrantAdmin:     canGrantAdmin,
		IsAdminCurrently:  isAdminCurrently,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "profile.html", data); err != nil {
		log.Printf("profile.html template: %v", err)
	}
}

func (h *ProfileHandler) ProfileUpcomingEvents(w http.ResponseWriter, r *http.Request) {
	h.renderProfileEventList(w, r, "upcoming")
}

func (h *ProfileHandler) ProfilePastEvents(w http.ResponseWriter, r *http.Request) {
	h.renderProfileEventList(w, r, "past")
}

func (h *ProfileHandler) renderProfileEventList(w http.ResponseWriter, r *http.Request, section string) {
	vars := muxVars(r)
	profileID := vars["id"]
	if profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	ctx := r.Context()
	var events []*event.ListItem
	var err error

	if section == "upcoming" {
		events, err = h.eventRepo.ListUpcomingByProfileID(ctx, profileID, 10, offset)
	} else {
		events, err = h.eventRepo.ListPastByProfileID(ctx, profileID, 10, offset)
	}
	if err != nil {
		log.Printf("List%sByProfileID: %v", section, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var allEvents []*event.ListItem
	if section == "upcoming" {
		allEvents, err = h.eventRepo.ListUpcomingByProfileID(ctx, profileID, 100000, 0)
	} else {
		allEvents, err = h.eventRepo.ListPastByProfileID(ctx, profileID, 100000, 0)
	}
	if err != nil {
		log.Printf("List%sByProfileID (all): %v", section, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	total := len(allEvents)

	displayed := offset + len(events)
	nextOffset := offset + 10
	hasMore := displayed < total

	data := profileEventListData{
		ProfileID:  profileID,
		Events:     events,
		Section:    section,
		Displayed:  displayed,
		Total:      total,
		NextOffset: nextOffset,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "profile_event_list.html", data); err != nil {
		log.Printf("profile_event_list.html template: %v", err)
	}
}

func (h *ProfileHandler) GrantAdmin(w http.ResponseWriter, r *http.Request) {
	h.toggleAdmin(w, r, true)
}

func (h *ProfileHandler) RemoveAdmin(w http.ResponseWriter, r *http.Request) {
	h.toggleAdmin(w, r, false)
}

func (h *ProfileHandler) toggleAdmin(w http.ResponseWriter, r *http.Request, grant bool) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := muxVars(r)
	profileID := vars["id"]
	if profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	p, err := h.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		log.Printf("toggleAdmin GetByID: %v", err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	if p.UserID == nil || *p.UserID == "" {
		http.Error(w, "Profile not registered", http.StatusBadRequest)
		return
	}

	targetUserID := *p.UserID

	if grant && targetUserID == user.ID {
		http.Error(w, "Cannot grant admin to yourself", http.StatusBadRequest)
		return
	}

	adminRole, err := h.rbac.GetRoleByName(ctx, "admin")
	if err != nil {
		log.Printf("get admin role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if grant {
		if err := h.rbac.AssignRoleToUser(ctx, targetUserID, adminRole.ID); err != nil {
			log.Printf("assign admin role: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.rbac.RemoveRoleFromUser(ctx, targetUserID, adminRole.ID); err != nil {
			log.Printf("remove admin role: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "profile_admin_section.html", map[string]any{
		"ProfileID":        profileID,
		"IsAdminCurrently": grant,
	}); err != nil {
		log.Printf("profile_admin_section.html template: %v", err)
	}
}


