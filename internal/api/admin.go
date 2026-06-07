package api

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"

	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
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
	tmpl                *template.Template
}

func NewAdminHandler(profileRepo profile.Repository, parentYouthLinkRepo parentyouthlink.Repository) *AdminHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &AdminHandler{
		profileRepo:         profileRepo,
		parentYouthLinkRepo: parentYouthLinkRepo,
		tmpl:                tmpl,
	}
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
		profileNames[p.ID] = p.FirstName + " " + p.LastName
	}

	var adults, youth []rosterRow
	for _, p := range allProfiles {
		if search != "" {
			needle := strings.ToLower(search)
			name := strings.ToLower(p.FirstName + " " + p.LastName)
			email := strings.ToLower(p.Email)
			if !strings.Contains(name, needle) && !strings.Contains(email, needle) {
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
			Name:    p.FirstName + " " + p.LastName,
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
