package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	stdSync "sync"
	"time"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/profile"
	domainsync "scout-app/internal/domain/sync"
	"scout-app/internal/scoutbook"
)

type tokenRequest struct {
	LoginData string `json:"login_data"`
}

// StoreToken accepts either:
//   - POST with Content-Type: application/json and body {"login_data":"..."}
//   - POST with Content-Type: application/x-www-form-urlencoded and field login_data=...

type storedToken struct {
	token      string
	personGUID string
	expiresAt  time.Time
}

type syncPageData struct {
	Title         string
	HasToken      bool
	PersonGUID    string
	OrgGUID       string
	Result        *domainsync.Result
	AdultProfiles []domainsync.ProfileReport
	YouthProfiles []domainsync.ProfileReport
	Error         string
}

type SyncHandler struct {
	svc           *domainsync.Service
	client        *scoutbook.Client
	tmpl          *template.Template
	orgGUID       string
	appConfigRepo appconfig.Repository

	mu    stdSync.RWMutex
	token *storedToken
}

func NewSyncHandler(svc *domainsync.Service, client *scoutbook.Client, appConfigRepo appconfig.Repository) *SyncHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &SyncHandler{
		svc:           svc,
		client:        client,
		tmpl:          tmpl,
		appConfigRepo: appConfigRepo,
	}
}

func (h *SyncHandler) loadOrgGUID(ctx context.Context) string {
	guid, _ := h.appConfigRepo.Get(ctx, appconfig.KeyScoutbookOrgGUID)
	if guid != h.orgGUID {
		h.orgGUID = guid
		h.client.SetOrgGUID(guid)
	}
	return h.orgGUID
}

func (h *SyncHandler) SetOrgGUID(orgGUID string) {
	h.orgGUID = orgGUID
	h.client.SetOrgGUID(orgGUID)
}

func (h *SyncHandler) syncPageData() syncPageData {
	return syncPageData{
		Title:   "Admin: Scoutbook Sync",
		OrgGUID: h.orgGUID,
	}
}

func (h *SyncHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	h.loadOrgGUID(r.Context())
	data := h.syncPageData()
	h.mu.RLock()
	data.HasToken = h.token != nil && h.token.expiresAt.After(time.Now())
	if data.HasToken {
		data.PersonGUID = h.token.personGUID
	}
	h.mu.RUnlock()

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		data.Error = errMsg
	}

	if r.Header.Get("HX-Request") != "" {
		h.renderPage(w, data)
		return
	}

	renderAdminLayout(w, h.tmpl, "admin_sync_content", data)
}

func (h *SyncHandler) renderPage(w http.ResponseWriter, data syncPageData) {
	if data.OrgGUID == "" {
		data.OrgGUID = h.orgGUID
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_sync_content", data); err != nil {
		log.Printf("admin_sync template: %v", err)
	}
}

func (h *SyncHandler) StoreToken(w http.ResponseWriter, r *http.Request) {
	h.loadOrgGUID(r.Context())
	raw := r.FormValue("login_data")
	if raw == "" {
		var req tokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("StoreToken decode: %v", err)
			data := h.syncPageData()
			data.Error = "Invalid request body"
			h.renderPage(w, data)
			return
		}
		raw = req.LoginData
	}

	if raw == "" {
		data := h.syncPageData()
		data.Error = "Missing login_data"
		h.renderPage(w, data)
		return
	}

	var parsed struct {
		Token      string `json:"token"`
		PersonGUID string `json:"personGuid"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Printf("StoreToken parse login_data: %v", err)
		data := h.syncPageData()
		data.Error = "Could not parse login_data as JSON. Paste the full LOGIN_DATA object from advancements.scouting.org localStorage."
		h.renderPage(w, data)
		return
	}

	if parsed.Token == "" || parsed.PersonGUID == "" {
		data := h.syncPageData()
		data.Error = "login_data must contain both token and personGuid fields."
		h.renderPage(w, data)
		return
	}

	exp := parseJWTExpiry(parsed.Token)

	h.mu.Lock()
	h.token = &storedToken{
		token:      parsed.Token,
		personGUID: parsed.PersonGUID,
		expiresAt:  exp,
	}
	h.mu.Unlock()

	h.client.SetToken(parsed.Token)

	data := h.syncPageData()
	data.HasToken = true
	data.PersonGUID = parsed.PersonGUID
	h.renderPage(w, data)
}

func (h *SyncHandler) Sync(w http.ResponseWriter, r *http.Request) {
	h.loadOrgGUID(r.Context())
	h.mu.RLock()
	st := h.token
	h.mu.RUnlock()

	if st == nil || st.token == "" {
		data := h.syncPageData()
		data.Error = "No Scoutbook token configured. Paste LOGIN_DATA first."
		h.renderPage(w, data)
		return
	}

	if time.Now().After(st.expiresAt) {
		h.mu.Lock()
		h.token = nil
		h.mu.Unlock()

		data := h.syncPageData()
		data.Error = "Token has expired. Please paste a new LOGIN_DATA."
		h.renderPage(w, data)
		return
	}

	h.client.SetToken(st.token)

	result, err := h.svc.Sync(r.Context())
	if err != nil {
		log.Printf("Sync failed: %v", err)
		data := h.syncPageData()
		data.HasToken = true
		data.PersonGUID = st.personGUID
		data.Error = err.Error()
		h.renderPage(w, data)
		return
	}

	data := h.syncPageData()
	data.HasToken = true
	data.PersonGUID = st.personGUID
	data.Result = result
	data.AdultProfiles = splitProfiles(result.Profiles, profile.MemberTypeAdult)
	data.YouthProfiles = splitProfiles(result.Profiles, profile.MemberTypeYouth)
	h.renderPage(w, data)
}

func (h *SyncHandler) Revert(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	memberID := r.FormValue("member_id")
	name := r.FormValue("name")

	var birthdate time.Time
	if bd := r.FormValue("old_birthdate"); bd != "" {
		birthdate, _ = time.Parse("2006-01-02", bd)
	}

	snapshot := domainsync.ProfileSnapshot{
		BSAID:      r.FormValue("old_bsa_id"),
		FirstName:  r.FormValue("old_first_name"),
		LastName:   r.FormValue("old_last_name"),
		Nickname:   r.FormValue("old_nickname"),
		Gender:     r.FormValue("old_gender"),
		Email:      r.FormValue("old_email"),
		Phone:      r.FormValue("old_phone"),
		Birthdate:  birthdate,
		MemberType: profile.MemberType(r.FormValue("old_member_type")),
		Status:     profile.Status(r.FormValue("old_status")),
		Positions:  r.FormValue("old_positions"),
	}

	if snapshot.BSAID == "" {
		log.Printf("Revert failed: empty BSAID for member %s", memberID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<div class="profile-card" id="profile-%s">`, memberID)
		_, _ = fmt.Fprintf(w, `<div class="error">Cannot revert: profile has no BSA ID. This profile may not have been imported from Scoutbook.</div>`)
		_, _ = fmt.Fprintf(w, `</div>`)
		return
	}

	addedRoles := r.Form["added_role"]
	removedRoles := r.Form["removed_role"]

	if err := h.svc.Revert(r.Context(), snapshot, addedRoles, removedRoles); err != nil {
		log.Printf("Revert failed: %v", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<div class="profile-card" id="profile-%s">`, memberID)
		_, _ = fmt.Fprintf(w, `<div class="error">Revert failed: %s</div>`, err.Error())
		_, _ = fmt.Fprintf(w, `</div>`)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	_ = t.ExecuteTemplate(w, "sync_reverted_card", map[string]string{
		"MemberID": memberID,
		"Name":     name,
	})
}

func splitProfiles(profiles []domainsync.ProfileReport, memberType profile.MemberType) []domainsync.ProfileReport {
	var result []domainsync.ProfileReport
	for _, p := range profiles {
		if p.New.MemberType == memberType {
			result = append(result, p)
		}
	}
	return result
}

func parseJWTExpiry(jwt string) time.Time {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return time.Now().Add(24 * time.Hour)
	}

	payload := parts[1]
	// JWT uses base64url (no padding)
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// Try with padding
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return time.Now().Add(24 * time.Hour)
		}
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp == 0 {
		return time.Now().Add(24 * time.Hour)
	}

	return time.Unix(claims.Exp, 0)
}
