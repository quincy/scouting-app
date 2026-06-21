package api

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

type rosterRow struct {
	ID      string
	Name    string
	Email   string
	Status  string
	Claimed bool
	Links   []string
	sortKey string
}

type adminPageData struct {
	Title  string
	Adults []rosterRow
	Youth  []rosterRow

	Search  string
	Claimed string
	Status  string
	Total   int
}

type AdminHandler struct {
	profileRepo         profile.Repository
	parentYouthLinkRepo parentyouthlink.Repository
	rbacRepo            rbac.Repository
	auth                *auth.AuthService
	tmpl                *template.Template
}

func NewAdminHandler(profileRepo profile.Repository, parentYouthLinkRepo parentyouthlink.Repository, rbacRepo rbac.Repository, auth *auth.AuthService) *AdminHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &AdminHandler{
		profileRepo:         profileRepo,
		parentYouthLinkRepo: parentYouthLinkRepo,
		rbacRepo:            rbacRepo,
		auth:                auth,
		tmpl:                tmpl,
	}
}

type adminRolesUserRow struct {
	ID       string
	Name     string
	Email    string
	Roles    string
	HasAdmin bool
	IsSelf   bool
}

type adminRolesPageData struct {
	Title  string
	Adults []adminRolesUserRow
	Youth  []adminRolesUserRow
	Total  int
}

func (h *AdminHandler) RolesPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildRolesData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") != "" {
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_roles", data); err != nil {
			log.Printf("admin_roles template: %v", err)
		}
		return
	}
	renderAdminLayout(w, h.tmpl, "admin_roles", data)
}

func (h *AdminHandler) GrantAdmin(w http.ResponseWriter, r *http.Request) {
	h.toggleAdmin(w, r, true)
}

func (h *AdminHandler) RemoveAdmin(w http.ResponseWriter, r *http.Request) {
	h.toggleAdmin(w, r, false)
}

func (h *AdminHandler) toggleAdmin(w http.ResponseWriter, r *http.Request, grant bool) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	targetUserID := extractIDFromPathRole(r.URL.Path)
	if targetUserID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if grant && targetUserID == user.ID {
		data := h.buildRolesData(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_roles", data); err != nil {
			log.Printf("admin_roles template: %v", err)
		}
		return
	}

	ctx := r.Context()

	adminRole, err := h.rbacRepo.GetRoleByName(ctx, "admin")
	if err != nil {
		log.Printf("get admin role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if grant {
		if err := h.rbacRepo.AssignRoleToUser(ctx, targetUserID, adminRole.ID); err != nil {
			log.Printf("assign admin role: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.rbacRepo.RemoveRoleFromUser(ctx, targetUserID, adminRole.ID); err != nil {
			log.Printf("remove admin role: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	data := h.buildRolesData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_roles", data); err != nil {
		log.Printf("admin_roles template: %v", err)
	}
}

func extractIDFromPathRole(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	for i, p := range parts {
		if p == "roles" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func (h *AdminHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildRosterData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderAdminLayout(w, h.tmpl, "admin_roster", data)
}

func (h *AdminHandler) RosterPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildRosterData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") != "" {
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_roster", data); err != nil {
			log.Printf("admin_roster template: %v", err)
		}
		return
	}
	renderAdminLayout(w, h.tmpl, "admin_roster", data)
}

func renderAdminLayout(w http.ResponseWriter, tmpl *template.Template, contentTmpl string, data any) {
	def := fmt.Sprintf(`{{define "content_panel"}}{{template "%s" .}}{{end}}`, contentTmpl)
	t := template.Must(template.Must(tmpl.Clone()).Parse(def))
	if err := t.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		log.Printf("admin_layout template: %v", err)
	}
}

type pendingLinkRow struct {
	ID          string
	ParentName  string
	YouthName   string
	YouthBSAID  string
	RequestedAt string
}

type activeConnectionRow struct {
	ID         string
	ParentName string
	YouthName  string
	YouthBSAID string
	Status     string
	ApprovedAt string
	ApprovedBy string
}

type adminConnectionsPageData struct {
	Title        string
	Pending      []pendingLinkRow
	Active       []activeConnectionRow
	Search       string
	PendingTotal int
	ActiveTotal  int
}

func (h *AdminHandler) ConnectionsPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildConnectionsData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") != "" {
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_connections", data); err != nil {
			log.Printf("admin_connections template: %v", err)
		}
		return
	}
	renderAdminLayout(w, h.tmpl, "admin_connections", data)
}

func (h *AdminHandler) ApproveConnection(w http.ResponseWriter, r *http.Request) {
	h.updateLinkStatus(w, r, parentyouthlink.StatusApproved)
}

func (h *AdminHandler) RejectConnection(w http.ResponseWriter, r *http.Request) {
	h.updateLinkStatus(w, r, parentyouthlink.StatusRejected)
}

func (h *AdminHandler) RemoveConnection(w http.ResponseWriter, r *http.Request) {
	h.updateLinkStatus(w, r, parentyouthlink.StatusRevoked)
}

func (h *AdminHandler) updateLinkStatus(w http.ResponseWriter, r *http.Request, newStatus parentyouthlink.Status) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id := extractIDFromPath(r.URL.Path)
	if id == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	link, err := h.parentYouthLinkRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if link == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if err := h.parentYouthLinkRepo.UpdateStatus(ctx, id, newStatus, user.ID); err != nil {
		log.Printf("UpdateStatus: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := h.buildConnectionsData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_connections", data); err != nil {
		log.Printf("admin_connections template: %v", err)
	}
}

func extractIDFromPath(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	for i, p := range parts {
		if p == "connections" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func (h *AdminHandler) buildConnectionsData(r *http.Request) adminConnectionsPageData {
	search := strings.ToLower(r.URL.Query().Get("search"))
	ctx := r.Context()

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		return adminConnectionsPageData{Title: "Admin: Connections"}
	}

	profileMap := make(map[string]*profile.Profile)
	for _, p := range allProfiles {
		profileMap[p.ID] = p
	}

	userIDToName := make(map[string]string)
	for _, p := range allProfiles {
		if p.UserID != nil {
			userIDToName[*p.UserID] = p.DisplayName()
		}
	}

	allLinks, err := h.parentYouthLinkRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll links: %v", err)
		return adminConnectionsPageData{Title: "Admin: Connections"}
	}

	var pending []pendingLinkRow
	var active []activeConnectionRow

	resolveName := func(id string) string {
		if p, ok := profileMap[id]; ok {
			return p.DisplayName()
		}
		return id
	}

	resolveBSAID := func(id string) string {
		if p, ok := profileMap[id]; ok {
			return p.BSAID
		}
		return ""
	}

	for _, link := range allLinks {
		switch link.Status {
		case parentyouthlink.StatusPending:
			pending = append(pending, pendingLinkRow{
				ID:          link.ID,
				ParentName:  resolveName(link.ParentProfileID),
				YouthName:   resolveName(link.YouthProfileID),
				YouthBSAID:  resolveBSAID(link.YouthProfileID),
				RequestedAt: link.RequestedAt.Format("Jan 2, 2006 3:04 PM"),
			})
		case parentyouthlink.StatusApproved, parentyouthlink.StatusRevoked:
			parentName := resolveName(link.ParentProfileID)
			youthName := resolveName(link.YouthProfileID)

			if search != "" {
				needle := strings.ToLower(search)
				if !strings.Contains(strings.ToLower(parentName), needle) &&
					!strings.Contains(strings.ToLower(youthName), needle) {
					continue
				}
			}

			approvedBy := ""
			if link.ApprovedBy != nil {
				if name, ok := userIDToName[*link.ApprovedBy]; ok {
					approvedBy = name
				} else {
					approvedBy = *link.ApprovedBy
				}
			}
			approvedAt := ""
			if link.ApprovedAt != nil {
				approvedAt = link.ApprovedAt.Format("Jan 2, 2006 3:04 PM")
			}

			displayStatus := "Active"
			if link.Status == parentyouthlink.StatusRevoked {
				displayStatus = "Revoked"
			}

			active = append(active, activeConnectionRow{
				ID:         link.ID,
				ParentName: parentName,
				YouthName:  youthName,
				YouthBSAID: resolveBSAID(link.YouthProfileID),
				Status:     displayStatus,
				ApprovedAt: approvedAt,
				ApprovedBy: approvedBy,
			})
		}
	}

	if pending == nil {
		pending = []pendingLinkRow{}
	}
	if active == nil {
		active = []activeConnectionRow{}
	}

	return adminConnectionsPageData{
		Title:        "Admin: Connections",
		Pending:      pending,
		Active:       active,
		Search:       r.URL.Query().Get("search"),
		PendingTotal: len(pending),
		ActiveTotal:  len(active),
	}
}

func (h *AdminHandler) buildRosterData(r *http.Request) adminPageData {
	search := r.URL.Query().Get("search")
	claimedFilter := r.URL.Query().Get("claimed")
	statusFilter := r.URL.Query().Get("status")

	ctx := r.Context()

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		return adminPageData{
			Title:   "Admin: Roster",
			Adults:  []rosterRow{},
			Youth:   []rosterRow{},
			Search:  search,
			Claimed: claimedFilter,
			Status:  statusFilter,
		}
	}

	allLinks, err := h.parentYouthLinkRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll links: %v", err)
		allLinks = nil
	}

	parentToYouth := make(map[string][]string)
	youthToParent := make(map[string]string)
	for _, link := range allLinks {
		if link.Status != parentyouthlink.StatusApproved {
			continue
		}
		parentToYouth[link.ParentProfileID] = append(parentToYouth[link.ParentProfileID], link.YouthProfileID)
		youthToParent[link.YouthProfileID] = link.ParentProfileID
	}

	profileNames := make(map[string]string)
	for _, p := range allProfiles {
		profileNames[p.ID] = p.DisplayName()
	}

	var adults, youth []rosterRow
	for _, p := range allProfiles {
		if search != "" {
			needle := strings.ToLower(search)
			displayName := strings.ToLower(p.DisplayName())
			email := strings.ToLower(p.Email)
			nickname := strings.ToLower(p.Nickname)
			if !strings.Contains(displayName, needle) && !strings.Contains(email, needle) && !strings.Contains(nickname, needle) {
				continue
			}
		}
		claimed := p.UserID != nil
		if claimedFilter == "true" && !claimed {
			continue
		}
		if claimedFilter == "false" && claimed {
			continue
		}
		if statusFilter != "" && string(p.Status) != statusFilter {
			continue
		}

		row := rosterRow{
			ID:      p.ID,
			Name:    p.DisplayName(),
			Email:   p.Email,
			Status:  string(p.Status),
			Claimed: claimed,
			sortKey: strings.ToLower(p.LastName + ", " + p.FirstName),
		}

		if p.MemberType == profile.MemberTypeAdult {
			for _, youthID := range parentToYouth[p.ID] {
				if name, ok := profileNames[youthID]; ok {
					row.Links = append(row.Links, name)
				}
			}
			adults = append(adults, row)
		} else {
			if parentID, ok := youthToParent[p.ID]; ok {
				if name, ok := profileNames[parentID]; ok {
					row.Links = append(row.Links, name)
				}
			}
			youth = append(youth, row)
		}
	}

	sort.Slice(adults, func(i, j int) bool {
		return adults[i].sortKey < adults[j].sortKey
	})
	sort.Slice(youth, func(i, j int) bool {
		return youth[i].sortKey < youth[j].sortKey
	})

	if adults == nil {
		adults = []rosterRow{}
	}
	if youth == nil {
		youth = []rosterRow{}
	}

	return adminPageData{
		Title:   "Admin: Roster",
		Adults:  adults,
		Youth:   youth,
		Search:  search,
		Claimed: claimedFilter,
		Status:  statusFilter,
		Total:   len(adults) + len(youth),
	}
}

func (h *AdminHandler) buildRolesData(r *http.Request) adminRolesPageData {
	ctx := r.Context()

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		return adminRolesPageData{Title: "Admin: Roles"}
	}

	allRoles, err := h.rbacRepo.ListAllRoles(ctx)
	if err != nil {
		log.Printf("ListAll roles: %v", err)
		return adminRolesPageData{Title: "Admin: Roles"}
	}

	adminRoleID := ""
	for _, role := range allRoles {
		if role.Name == "admin" {
			adminRoleID = role.ID
			break
		}
	}

	currentUser, err := h.auth.GetAuthenticatedUser(r)
	currentUserID := ""
	if err == nil && currentUser != nil {
		currentUserID = currentUser.ID
	}

	buildRow := func(p *profile.Profile) (adminRolesUserRow, bool) {
		row := adminRolesUserRow{
			Name:  p.DisplayName(),
			Email: p.Email,
		}

		if p.UserID == nil {
			row.Roles = "(not registered)"
			return row, true
		}

		row.ID = *p.UserID
		row.IsSelf = *p.UserID == currentUserID

		userRoles, err := h.rbacRepo.GetUserRoles(ctx, *p.UserID)
		if err != nil {
			log.Printf("GetUserRoles for %s: %v", *p.UserID, err)
			return row, false
		}

		roleNames := make([]string, 0, len(userRoles))
		for _, role := range userRoles {
			roleNames = append(roleNames, role.Name)
			if role.ID == adminRoleID {
				row.HasAdmin = true
			}
		}
		sort.Strings(roleNames)
		row.Roles = strings.Join(roleNames, ", ")
		if row.Roles == "" {
			row.Roles = "(none)"
		}
		return row, true
	}

	var adults, youth []adminRolesUserRow
	for _, p := range allProfiles {
		row, ok := buildRow(p)
		if !ok {
			continue
		}
		if p.MemberType == profile.MemberTypeAdult {
			adults = append(adults, row)
		} else {
			youth = append(youth, row)
		}
	}

	sort.Slice(adults, func(i, j int) bool { return adults[i].Name < adults[j].Name })
	sort.Slice(youth, func(i, j int) bool { return youth[i].Name < youth[j].Name })

	if adults == nil {
		adults = []adminRolesUserRow{}
	}
	if youth == nil {
		youth = []adminRolesUserRow{}
	}

	return adminRolesPageData{
		Title:  "Admin: Roles",
		Adults: adults,
		Youth:  youth,
		Total:  len(adults) + len(youth),
	}
}
