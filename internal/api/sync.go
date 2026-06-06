package api

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	stdSync "sync"
	"time"

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
	Title      string
	HasToken   bool
	PersonGUID string
	OrgGUID    string
	Result     *domainsync.Result
	Error      string
}

type SyncHandler struct {
	svc    *domainsync.Service
	client *scoutbook.Client
	tmpl   *template.Template

	mu    stdSync.RWMutex
	token *storedToken
}

func NewSyncHandler(svc *domainsync.Service, client *scoutbook.Client) *SyncHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &SyncHandler{
		svc:    svc,
		client: client,
		tmpl:   tmpl,
	}
}

func (h *SyncHandler) syncPageData() syncPageData {
	return syncPageData{
		Title:   "Admin: Scoutbook Sync",
		OrgGUID: h.client.OrgGUID(),
	}
}

func (h *SyncHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	hasToken := h.token != nil && h.token.expiresAt.After(time.Now())
	personGUID := ""
	if hasToken {
		personGUID = h.token.personGUID
	}
	h.mu.RUnlock()

	data := h.syncPageData()
	data.HasToken = hasToken
	data.PersonGUID = personGUID
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		data.Error = errMsg
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "admin_sync.html", data); err != nil {
		log.Printf("admin_sync template: %v", err)
	}
}

func (h *SyncHandler) renderPage(w http.ResponseWriter, data syncPageData) {
	if data.OrgGUID == "" {
		data.OrgGUID = h.client.OrgGUID()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "admin_sync_content", data); err != nil {
		log.Printf("admin_sync template: %v", err)
	}
}

func (h *SyncHandler) StoreToken(w http.ResponseWriter, r *http.Request) {
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
	h.renderPage(w, data)
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
