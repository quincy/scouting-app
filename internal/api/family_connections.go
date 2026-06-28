package api

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/email"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

type FamilyConnectionsHandler struct {
	profileRepo profile.Repository
	linkRepo    parentyouthlink.Repository
	auth        *auth.AuthService
	rbac        rbac.Repository
	emailSvc    email.Service
	tmpl        *template.Template
}

type connectionVM struct {
	OtherName     string
	OtherInactive bool
	Status        string
	RequestedAt   string
}

type familyConnectionsPageData struct {
	Title       string
	IsAdmin     bool
	Connections []connectionVM
	IsAdult     bool
	Error       string
	Success     string
}

func NewFamilyConnectionsHandler(
	profileRepo profile.Repository,
	linkRepo parentyouthlink.Repository,
	auth *auth.AuthService,
	rbac rbac.Repository,
	emailSvc email.Service,
) *FamilyConnectionsHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &FamilyConnectionsHandler{
		profileRepo: profileRepo,
		linkRepo:    linkRepo,
		auth:        auth,
		rbac:        rbac,
		emailSvc:    emailSvc,
		tmpl:        tmpl,
	}
}

func (h *FamilyConnectionsHandler) isAdmin(ctx context.Context, r *http.Request) bool {
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

func (h *FamilyConnectionsHandler) notifyAdmins(ctx context.Context, parentProfile, youthProfile *profile.Profile) {
	adminUserIDs, err := h.rbac.GetUsersByRoleName(ctx, "admin")
	if err != nil {
		log.Printf("notifyAdmins: GetUsersByRoleName: %v", err)
		return
	}

	var adminEmails []string
	for _, uid := range adminUserIDs {
		p, err := h.profileRepo.GetByUserID(ctx, uid)
		if err != nil {
			log.Printf("notifyAdmins: GetByUserID %s: %v", uid, err)
			continue
		}
		if p.Email != "" {
			adminEmails = append(adminEmails, p.Email)
		}
	}

	if len(adminEmails) == 0 {
		return
	}

	subject := "New Family Connection Request"
	body := "A new family connection request has been submitted and is awaiting your review.\n\n" +
		"Please visit the admin panel to review and approve or reject the request:\n" +
		"http://localhost:8080/admin/connections"

	if err := h.emailSvc.SendAdminNotification(ctx, adminEmails, subject, body); err != nil {
		log.Printf("notifyAdmins: SendAdminNotification: %v", err)
	}
}

func (h *FamilyConnectionsHandler) renderPage(w http.ResponseWriter, r *http.Request, data familyConnectionsPageData) {
	data.Title = "Family Connections"
	data.IsAdmin = h.isAdmin(r.Context(), r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "family_connections.html", data); err != nil {
		log.Printf("family_connections template execution: %v", err)
	}
}

func (h *FamilyConnectionsHandler) buildConnections(ctx context.Context, profileID string, isAdult bool) []connectionVM {
	var conns []*parentyouthlink.ParentYouthConnection

	if isAdult {
		links, err := h.linkRepo.ListByParent(ctx, profileID)
		if err == nil {
			conns = append(conns, links...)
		}
	} else {
		links, err := h.linkRepo.ListByYouth(ctx, profileID)
		if err == nil {
			conns = append(conns, links...)
		}
	}

	var vms []connectionVM
	for _, c := range conns {
		var otherProfile *profile.Profile
		var err error
		if isAdult {
			otherProfile, err = h.profileRepo.GetByID(ctx, c.YouthProfileID)
		} else {
			otherProfile, err = h.profileRepo.GetByID(ctx, c.ParentProfileID)
		}
		if err != nil {
			continue
		}
		vms = append(vms, connectionVM{
			OtherName:     otherProfile.DisplayName(),
			OtherInactive: otherProfile.Status == profile.StatusInactive,
			Status:        string(c.Status),
			RequestedAt:   c.RequestedAt.Format("Jan 2, 2006"),
		})
	}
	return vms
}

// GET /family-connections
func (h *FamilyConnectionsHandler) FamilyConnectionsPage(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login?redirect=/family-connections", http.StatusFound)
		return
	}

	ctx := r.Context()
	userProfile, err := h.profileRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		log.Printf("GetByUserID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isAdult := userProfile.MemberType == profile.MemberTypeAdult
	conns := h.buildConnections(ctx, userProfile.ID, isAdult)

	h.renderPage(w, r, familyConnectionsPageData{
		Connections: conns,
		IsAdult:     isAdult,
	})
}

// POST /family-connections
func (h *FamilyConnectionsHandler) AddConnection(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login?redirect=/family-connections", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	bsaID := strings.TrimSpace(r.FormValue("bsa_id"))
	if bsaID == "" {
		h.renderPage(w, r, familyConnectionsPageData{
			Error: "Please enter the youth's BSA ID.",
		})
		return
	}

	ctx := r.Context()

	parentProfile, err := h.profileRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		log.Printf("GetByUserID: %v", err)
		h.renderPage(w, r, familyConnectionsPageData{Error: "Could not find your profile. Please contact support."})
		return
	}

	isAdult := parentProfile.MemberType == profile.MemberTypeAdult

	youthProfile, err := h.profileRepo.GetByBSAID(ctx, bsaID)
	if err != nil {
		conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
		h.renderPage(w, r, familyConnectionsPageData{
			Connections: conns,
			IsAdult:     isAdult,
			Error:       "No profile found with that BSA ID.",
		})
		return
	}

	if youthProfile.MemberType != profile.MemberTypeYouth {
		conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
		h.renderPage(w, r, familyConnectionsPageData{
			Connections: conns,
			IsAdult:     isAdult,
			Error:       "That BSA ID belongs to an adult, not a youth.",
		})
		return
	}

	if youthProfile.UserID != nil {
		conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
		h.renderPage(w, r, familyConnectionsPageData{
			Connections: conns,
			IsAdult:     isAdult,
			Error:       "This youth already has a linked account.",
		})
		return
	}

	existingLinks, err := h.linkRepo.ListByParent(ctx, parentProfile.ID)
	if err == nil {
		for _, l := range existingLinks {
			if l.YouthProfileID == youthProfile.ID && l.Status == parentyouthlink.StatusPending {
				conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
				h.renderPage(w, r, familyConnectionsPageData{
					Connections: conns,
					IsAdult:     isAdult,
					Error:       "You already have a pending request for this youth.",
				})
				return
			}
		}
	}

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parentProfile.ID,
		YouthProfileID:  youthProfile.ID,
		Status:          parentyouthlink.StatusPending,
		RequestedAt:     time.Now(),
	}
	if err := h.linkRepo.Create(ctx, link); err != nil {
		log.Printf("Create link: %v", err)
		conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
		h.renderPage(w, r, familyConnectionsPageData{
			Connections: conns,
			IsAdult:     isAdult,
			Error:       "An error occurred. Please try again.",
		})
		return
	}

	h.notifyAdmins(ctx, parentProfile, youthProfile)

	conns := h.buildConnections(ctx, parentProfile.ID, isAdult)
	h.renderPage(w, r, familyConnectionsPageData{
		Connections: conns,
		IsAdult:     isAdult,
		Success:     "Request sent! An admin will review the link request.",
	})
}
