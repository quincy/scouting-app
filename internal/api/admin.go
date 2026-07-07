package api

import (
	"context"
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
	ID         string
	Name       string
	Email      string
	BSAID      string
	Status     string
	Registered bool
	IsSelf     bool
	Links      []string
	sortKey    string
}

type adminPageData struct {
	Title  string
	Adults []rosterRow
	Youth  []rosterRow

	Search     string
	Registered string
	Status     string
	Total      int
	AdminPerms
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

type rolePermissionRow struct {
	RoleID      string
	RoleName    string
	Permissions string
	UserCount   int
	UserNames   string
}

type permissionCheckbox struct {
	ID       string
	Name     string
	Checked  bool
	Disabled bool
}

type rolePermissionModalData struct {
	RoleID      string
	RoleName    string
	Permissions []permissionCheckbox
}

type adminRolesPageData struct {
	Title      string
	AdultRoles []rolePermissionRow
	YouthRoles []rolePermissionRow
	CloseModal bool
	AdminPerms
}

func isYouthRole(name string) bool {
	switch name {
	case "Scouts BSA",
		"Assistant Patrol Leader",
		"Assistant Senior Patrol Leader",
		"Chaplain Aide",
		"Den Chief",
		"Historian",
		"Librarian",
		"OA Unit Representative",
		"Outdoor Ethics Guide",
		"Patrol Admin",
		"Patrol Leader",
		"Quartermaster",
		"Scribe",
		"Senior Patrol Leader",
		"Troop Guide",
		"Webmaster":
		return true
	}
	return false
}

func (h *AdminHandler) RolesPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildRolesData(r)
	if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
		data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	}
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

func (h *AdminHandler) RolesEditModal(w http.ResponseWriter, r *http.Request) {
	roleID := muxVars(r)["id"]
	if roleID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	roles, err := h.rbacRepo.ListAllRoles(ctx)
	if err != nil {
		log.Printf("ListAllRoles: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var role *rbac.Role
	for _, rl := range roles {
		if rl.ID == roleID {
			role = rl
			break
		}
	}
	if role == nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	allPerms, err := h.rbacRepo.ListAllPermissions(ctx)
	if err != nil {
		log.Printf("ListAllPermissions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rolePerms, err := h.rbacRepo.GetRolePermissions(ctx, roleID)
	if err != nil {
		log.Printf("GetRolePermissions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rolePermSet := make(map[string]bool)
	for _, p := range rolePerms {
		rolePermSet[p.ID] = true
	}

	isAdminRole := role.Name == "admin"

	var checkboxes []permissionCheckbox
	for _, p := range allPerms {
		disabled := isAdminRole && p.Name == "admin:rbac"
		checkboxes = append(checkboxes, permissionCheckbox{
			ID:       p.ID,
			Name:     p.Name,
			Checked:  rolePermSet[p.ID] || disabled,
			Disabled: disabled,
		})
	}

	data := rolePermissionModalData{
		RoleID:      roleID,
		RoleName:    role.Name,
		Permissions: checkboxes,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_roles_modal", data); err != nil {
		log.Printf("admin_roles_modal template: %v", err)
	}
}

func (h *AdminHandler) RolesSavePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := muxVars(r)["id"]
	if roleID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	permIDs := r.Form["permissions"]

	ctx := r.Context()
	if err := h.rbacRepo.SetRolePermissions(ctx, roleID, permIDs); err != nil {
		log.Printf("SetRolePermissions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := h.buildRolesData(r)
	data.CloseModal = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_roles", data); err != nil {
		log.Printf("admin_roles template: %v", err)
	}
}

func (h *AdminHandler) RolesUsersModal(w http.ResponseWriter, r *http.Request) {
	roleID := muxVars(r)["id"]
	if roleID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	roles, err := h.rbacRepo.ListAllRoles(ctx)
	if err != nil {
		log.Printf("ListAllRoles: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var role *rbac.Role
	for _, rl := range roles {
		if rl.ID == roleID {
			role = rl
			break
		}
	}
	if role == nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userIDToName := make(map[string]string)
	for _, p := range allProfiles {
		if p.UserID != nil {
			userIDToName[*p.UserID] = p.DisplayName()
		}
	}

	userIDs, err := h.rbacRepo.GetUsersByRoleName(ctx, role.Name)
	if err != nil {
		log.Printf("GetUsersByRoleName: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var userNames []string
	for _, uid := range userIDs {
		if name, ok := userIDToName[uid]; ok {
			userNames = append(userNames, name)
		} else {
			userNames = append(userNames, uid)
		}
	}
	sort.Strings(userNames)

	data := struct {
		RoleName string
		Users    []string
	}{
		RoleName: role.Name,
		Users:    userNames,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_roles_users_modal", data); err != nil {
		log.Printf("admin_roles_users_modal template: %v", err)
	}
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

	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	targetUserID := ""
	for i, p := range parts {
		if p == "roles" && i+1 < len(parts) {
			targetUserID = parts[i+1]
			break
		}
	}
	if targetUserID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if grant && targetUserID == user.ID {
		data := h.buildRolesData(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("HX-Trigger-After-Swap", "closeModal")
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

func (h *AdminHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login?redirect=/admin", http.StatusFound)
		return
	}

	perms := computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	redirect := perms.FirstAvailable()
	if redirect == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (h *AdminHandler) RosterPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildRosterData(r)
	if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
		data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	}
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

func (h *AdminHandler) ToggleProfileStatus(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	profileID := muxVars(r)["id"]
	if profileID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	p, err := h.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		log.Printf("GetByID: %v", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if p.Status == profile.StatusActive {
		p.Status = profile.StatusInactive
	} else {
		p.Status = profile.StatusActive
	}

	if err := h.profileRepo.Update(ctx, p); err != nil {
		log.Printf("Update profile: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := h.buildRosterData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_roster", data); err != nil {
		log.Printf("admin_roster template: %v", err)
	}
}

type AdminPerms struct {
	CanAccessRoster      bool
	CanAccessConnections bool
	CanAccessSync        bool
	CanAccessRBAC        bool
	CanAccessSettings    bool
}

func (a AdminPerms) FirstAvailable() string {
	switch {
	case a.CanAccessRoster:
		return "/admin/roster"
	case a.CanAccessRBAC:
		return "/admin/roles"
	case a.CanAccessConnections:
		return "/admin/connections"
	case a.CanAccessSync:
		return "/admin/sync"
	case a.CanAccessSettings:
		return "/admin/settings"
	}
	return ""
}

func computeAdminPerms(ctx context.Context, rbacRepo rbac.Repository, userID string) AdminPerms {
	perms, err := rbacRepo.GetUserPermissions(ctx, userID)
	if err != nil {
		log.Printf("computeAdminPerms: %v", err)
		return AdminPerms{}
	}
	var a AdminPerms
	for _, p := range perms {
		switch p.Name {
		case "admin:roster":
			a.CanAccessRoster = true
		case "admin:connections":
			a.CanAccessConnections = true
		case "admin:sync":
			a.CanAccessSync = true
		case "admin:rbac":
			a.CanAccessRBAC = true
		case "admin:settings":
			a.CanAccessSettings = true
		}
	}
	return a
}

func renderAdminLayout(w http.ResponseWriter, tmpl *template.Template, contentTmpl string, data any) {
	def := fmt.Sprintf(`{{define "content_panel"}}{{template "%s" .}}{{end}}`, contentTmpl)
	t := template.Must(template.Must(tmpl.Clone()).Parse(def))
	if err := t.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		log.Printf("admin_layout template: %v", err)
	}
}

type pendingLinkRow struct {
	ID             string
	ParentName     string
	YouthName      string
	YouthBSAID     string
	RequestedAt    string
	ParentInactive bool
	YouthInactive  bool
}

type activeConnectionRow struct {
	ID             string
	ParentName     string
	YouthName      string
	YouthBSAID     string
	Status         string
	ApprovedAt     string
	ApprovedBy     string
	ParentInactive bool
	YouthInactive  bool
}

type adminConnectionsPageData struct {
	Title        string
	Pending      []pendingLinkRow
	Active       []activeConnectionRow
	Search       string
	PendingTotal int
	ActiveTotal  int
	AdminPerms
}

func (h *AdminHandler) ConnectionsPage(w http.ResponseWriter, r *http.Request) {
	data := h.buildConnectionsData(r)
	if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
		data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	}
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

	isInactive := func(id string) bool {
		if p, ok := profileMap[id]; ok {
			return p.Status == profile.StatusInactive
		}
		return false
	}

	for _, link := range allLinks {
		switch link.Status {
		case parentyouthlink.StatusPending:
			pending = append(pending, pendingLinkRow{
				ID:             link.ID,
				ParentName:     resolveName(link.ParentProfileID),
				YouthName:      resolveName(link.YouthProfileID),
				YouthBSAID:     resolveBSAID(link.YouthProfileID),
				RequestedAt:    link.RequestedAt.Format("Jan 2, 2006 3:04 PM"),
				ParentInactive: isInactive(link.ParentProfileID),
				YouthInactive:  isInactive(link.YouthProfileID),
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
				ID:             link.ID,
				ParentName:     parentName,
				YouthName:      youthName,
				YouthBSAID:     resolveBSAID(link.YouthProfileID),
				Status:         displayStatus,
				ApprovedAt:     approvedAt,
				ApprovedBy:     approvedBy,
				ParentInactive: isInactive(link.ParentProfileID),
				YouthInactive:  isInactive(link.YouthProfileID),
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
	registeredFilter := r.URL.Query().Get("registered")
	statusFilter := r.URL.Query().Get("status")

	ctx := r.Context()

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		return adminPageData{
			Title:      "Admin: Roster",
			Adults:     []rosterRow{},
			Youth:      []rosterRow{},
			Search:     search,
			Registered: registeredFilter,
			Status:     statusFilter,
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

	currentUserID := ""
	if u, e := h.auth.GetAuthenticatedUser(r); e == nil && u != nil {
		currentUserID = u.ID
	}

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
		registered := p.UserID != nil
		if registeredFilter == "true" && !registered {
			continue
		}
		if registeredFilter == "false" && registered {
			continue
		}
		if statusFilter != "" && string(p.Status) != statusFilter {
			continue
		}

		row := rosterRow{
			ID:         p.ID,
			Name:       p.DisplayName(),
			Email:      p.Email,
			BSAID:      p.BSAID,
			Status:     string(p.Status),
			Registered: registered,
			IsSelf:     p.UserID != nil && *p.UserID == currentUserID,
			sortKey:    strings.ToLower(p.LastName + ", " + p.FirstName),
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
		Title:      "Admin: Roster",
		Adults:     adults,
		Youth:      youth,
		Search:     search,
		Registered: registeredFilter,
		Status:     statusFilter,
		Total:      len(adults) + len(youth),
	}
}

func (h *AdminHandler) buildRolesData(r *http.Request) adminRolesPageData {
	ctx := r.Context()

	allProfiles, err := h.profileRepo.ListAll(ctx)
	if err != nil {
		log.Printf("ListAll profiles: %v", err)
		return adminRolesPageData{Title: "Admin: Roles & Permissions"}
	}

	userIDToName := make(map[string]string)
	activeUserID := make(map[string]bool)
	for _, p := range allProfiles {
		if p.UserID != nil {
			userIDToName[*p.UserID] = p.DisplayName()
			if p.Status == profile.StatusActive {
				activeUserID[*p.UserID] = true
			}
		}
	}

	allRoles, err := h.rbacRepo.ListAllRoles(ctx)
	if err != nil {
		log.Printf("ListAll roles: %v", err)
		return adminRolesPageData{Title: "Admin: Roles & Permissions"}
	}

	var adultRows, youthRows []rolePermissionRow
	for _, role := range allRoles {
		perms, err := h.rbacRepo.GetRolePermissions(ctx, role.ID)
		if err != nil {
			log.Printf("GetRolePermissions for %s: %v", role.Name, err)
			continue
		}

		permNames := make([]string, 0, len(perms))
		for _, p := range perms {
			permNames = append(permNames, p.Name)
		}
		sort.Strings(permNames)

		permStr := strings.Join(permNames, ", ")
		if permStr == "" {
			permStr = "(none)"
		}

		userIDs, err := h.rbacRepo.GetUsersByRoleName(ctx, role.Name)
		userCount := 0
		var userNames []string
		if err == nil {
			for _, uid := range userIDs {
				if !activeUserID[uid] {
					continue
				}
				userCount++
				if name, ok := userIDToName[uid]; ok {
					userNames = append(userNames, name)
				}
			}
		}
		sort.Strings(userNames)

		row := rolePermissionRow{
			RoleID:      role.ID,
			RoleName:    role.Name,
			Permissions: permStr,
			UserCount:   userCount,
			UserNames:   strings.Join(userNames, ", "),
		}

		if isYouthRole(role.Name) {
			youthRows = append(youthRows, row)
		} else {
			adultRows = append(adultRows, row)
		}
	}

	sort.Slice(adultRows, func(i, j int) bool { return adultRows[i].RoleName < adultRows[j].RoleName })
	sort.Slice(youthRows, func(i, j int) bool { return youthRows[i].RoleName < youthRows[j].RoleName })

	if adultRows == nil {
		adultRows = []rolePermissionRow{}
	}
	if youthRows == nil {
		youthRows = []rolePermissionRow{}
	}

	return adminRolesPageData{
		Title:      "Admin: Roles & Permissions",
		AdultRoles: adultRows,
		YouthRoles: youthRows,
	}
}
