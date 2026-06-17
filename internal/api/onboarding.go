package api

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strings"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/user"

	"github.com/gorilla/sessions"
)

const onboardingSessionName = "onboarding"

type timezoneOption struct {
	Value    string
	Label    string
	Selected bool
}

type onboardingFormData struct {
	Step       int
	TotalSteps int
	FirstName  string
	LastName   string
	Email      string
	BSAID      string
	UnitType   string
	UnitNumber string
	OrgGUID    string
	Timezone   string
	Timezones  []timezoneOption
	Error      string
}

type OnboardingHandler struct {
	profileRepo   profile.Repository
	userRepo      user.Repository
	rbacRepo      rbac.Repository
	appConfigRepo appconfig.Repository
	hasher        auth.Hasher
	session       *sessions.CookieStore
	tmpl          *template.Template
}

func NewOnboardingHandler(
	profileRepo profile.Repository,
	userRepo user.Repository,
	rbacRepo rbac.Repository,
	appConfigRepo appconfig.Repository,
	hasher auth.Hasher,
	session *sessions.CookieStore,
) *OnboardingHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS,
			"views/onboarding_welcome.html",
			"views/onboarding_personal.html",
			"views/onboarding_unit.html",
			"views/onboarding_timezone.html",
			"views/onboarding_password.html",
			"views/onboarding_complete.html",
		),
	)
	return &OnboardingHandler{
		profileRepo:   profileRepo,
		userRepo:      userRepo,
		rbacRepo:      rbacRepo,
		appConfigRepo: appConfigRepo,
		hasher:        hasher,
		session:       session,
		tmpl:          tmpl,
	}
}

var commonTimezones = []timezoneOption{
	{Value: "America/New_York", Label: "Eastern Time (US & Canada)"},
	{Value: "America/Chicago", Label: "Central Time (US & Canada)"},
	{Value: "America/Denver", Label: "Mountain Time (US & Canada)"},
	{Value: "America/Phoenix", Label: "Arizona (no DST)"},
	{Value: "America/Los_Angeles", Label: "Pacific Time (US & Canada)"},
	{Value: "America/Anchorage", Label: "Alaska"},
	{Value: "Pacific/Honolulu", Label: "Hawaii (no DST)"},
	{Value: "America/St_Johns", Label: "Newfoundland"},
	{Value: "America/Halifax", Label: "Atlantic Time (Canada)"},
	{Value: "Europe/London", Label: "London (GMT/BST)"},
	{Value: "Europe/Berlin", Label: "Berlin (CET/CEST)"},
	{Value: "UTC", Label: "UTC (no DST)"},
}

func (h *OnboardingHandler) WelcomePage(w http.ResponseWriter, r *http.Request) {
	h.clearForm(w, r)
	data := onboardingFormData{}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_welcome.html", data); err != nil {
		log.Printf("onboarding welcome template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) PersonalPage(w http.ResponseWriter, r *http.Request) {
	data := onboardingFormData{}
	data.Step = 1
	data.TotalSteps = 4
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_personal.html", data); err != nil {
		log.Printf("onboarding personal template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) Personal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	bsaID := strings.TrimSpace(r.FormValue("bsa_id"))

	data := onboardingFormData{
		Step:       1,
		TotalSteps: 4,
		FirstName:  firstName,
		LastName:   lastName,
		Email:      email,
		BSAID:      bsaID,
	}

	if firstName == "" || lastName == "" || email == "" || bsaID == "" {
		data.Error = "All fields are required."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_personal.html", data); err != nil {
			log.Printf("onboarding personal template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.saveForm(w, r, data)
	http.Redirect(w, r, "/onboard/unit", http.StatusFound)
}

func (h *OnboardingHandler) UnitPage(w http.ResponseWriter, r *http.Request) {
	data := h.loadForm(r)
	if data.FirstName == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}
	data.Step = 2
	data.TotalSteps = 4
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_unit.html", data); err != nil {
		log.Printf("onboarding unit template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) Unit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	prev := h.loadForm(r)
	if prev.FirstName == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}

	unitType := strings.TrimSpace(r.FormValue("unit_type"))
	unitNumber := strings.TrimSpace(r.FormValue("unit_number"))
	orgGUID := strings.TrimSpace(r.FormValue("org_guid"))

	data := onboardingFormData{
		Step:       2,
		TotalSteps: 4,
		FirstName:  prev.FirstName,
		LastName:   prev.LastName,
		Email:      prev.Email,
		BSAID:      prev.BSAID,
		UnitType:   unitType,
		UnitNumber: unitNumber,
		OrgGUID:    orgGUID,
	}

	if unitType == "" || unitNumber == "" || orgGUID == "" {
		data.Error = "All fields are required."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_unit.html", data); err != nil {
			log.Printf("onboarding unit template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.saveForm(w, r, data)
	http.Redirect(w, r, "/onboard/timezone", http.StatusFound)
}

func (h *OnboardingHandler) TimezonePage(w http.ResponseWriter, r *http.Request) {
	data := h.loadForm(r)
	if data.FirstName == "" || data.UnitType == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}
	data.Step = 3
	data.TotalSteps = 4
	data.Timezones = h.timezonesWithSelected(data.Timezone)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_timezone.html", data); err != nil {
		log.Printf("onboarding timezone template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) Timezone(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	prev := h.loadForm(r)
	if prev.FirstName == "" || prev.UnitType == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}

	timezone := strings.TrimSpace(r.FormValue("timezone"))

	data := prev
	data.Timezone = timezone
	data.Step = 3
	data.TotalSteps = 4

	if timezone == "" {
		data.Error = "Please select a timezone."
		data.Timezones = h.timezonesWithSelected(timezone)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_timezone.html", data); err != nil {
			log.Printf("onboarding timezone template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.saveForm(w, r, data)
	http.Redirect(w, r, "/onboard/password", http.StatusFound)
}

func (h *OnboardingHandler) PasswordPage(w http.ResponseWriter, r *http.Request) {
	data := h.loadForm(r)
	if data.FirstName == "" || data.UnitType == "" || data.Timezone == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}
	data.Step = 4
	data.TotalSteps = 4
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_password.html", data); err != nil {
		log.Printf("onboarding password template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) Password(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	prev := h.loadForm(r)
	if prev.FirstName == "" || prev.UnitType == "" || prev.Timezone == "" {
		http.Redirect(w, r, "/onboard", http.StatusFound)
		return
	}

	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	data := prev
	data.Step = 4
	data.TotalSteps = 4

	if password == "" {
		data.Error = "Please enter a password."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_password.html", data); err != nil {
			log.Printf("onboarding password template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if len(password) < 8 {
		data.Error = "Password must be at least 8 characters."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_password.html", data); err != nil {
			log.Printf("onboarding password template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if password != confirmPassword {
		data.Error = "Passwords do not match."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "onboarding_password.html", data); err != nil {
			log.Printf("onboarding password template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	ctx := r.Context()

	hash, err := h.hasher.Hash(password)
	if err != nil {
		log.Printf("hash password: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	u := &user.User{PasswordHash: hash}
	if err := h.userRepo.Create(ctx, u); err != nil {
		log.Printf("create user: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	adminRole, err := h.rbacRepo.GetRoleByName(ctx, "admin")
	if err != nil {
		log.Printf("get admin role: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.rbacRepo.AssignRoleToUser(ctx, u.ID, adminRole.ID); err != nil {
		log.Printf("assign admin role: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	prof := &profile.Profile{
		FirstName:  data.FirstName,
		LastName:   data.LastName,
		Email:      data.Email,
		BSAID:      data.BSAID,
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &u.ID,
	}
	if err := h.profileRepo.Create(ctx, prof); err != nil {
		log.Printf("create profile: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.appConfigRepo.Set(ctx, appconfig.KeyScoutbookOrgGUID, data.OrgGUID); err != nil {
		log.Printf("save org guid: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyUnitType, data.UnitType); err != nil {
		log.Printf("save unit type: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyUnitNumber, data.UnitNumber); err != nil {
		log.Printf("save unit number: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyDefaultTimezone, data.Timezone); err != nil {
		log.Printf("save timezone: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyOnboardingComplete, "true"); err != nil {
		log.Printf("save onboarding complete: %v", err)
		h.clearForm(w, r)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.clearForm(w, r)

	http.Redirect(w, r, "/onboard/complete", http.StatusFound)
}

func (h *OnboardingHandler) CompletePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "onboarding_complete.html", nil); err != nil {
		log.Printf("onboarding complete template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OnboardingHandler) loadForm(r *http.Request) onboardingFormData {
	sess, err := h.session.Get(r, onboardingSessionName)
	if err != nil {
		return onboardingFormData{}
	}
	return onboardingFormData{
		FirstName:  stringOr(sess.Values["first_name"]),
		LastName:   stringOr(sess.Values["last_name"]),
		Email:      stringOr(sess.Values["email"]),
		BSAID:      stringOr(sess.Values["bsa_id"]),
		UnitType:   stringOr(sess.Values["unit_type"]),
		UnitNumber: stringOr(sess.Values["unit_number"]),
		OrgGUID:    stringOr(sess.Values["org_guid"]),
		Timezone:   stringOr(sess.Values["timezone"]),
	}
}

func (h *OnboardingHandler) saveForm(w http.ResponseWriter, r *http.Request, data onboardingFormData) {
	sess, err := h.session.Get(r, onboardingSessionName)
	if err != nil {
		return
	}
	sess.Values["first_name"] = data.FirstName
	sess.Values["last_name"] = data.LastName
	sess.Values["email"] = data.Email
	sess.Values["bsa_id"] = data.BSAID
	sess.Values["unit_type"] = data.UnitType
	sess.Values["unit_number"] = data.UnitNumber
	sess.Values["org_guid"] = data.OrgGUID
	sess.Values["timezone"] = data.Timezone
	if err := sess.Save(r, w); err != nil {
		log.Printf("onboarding session save: %v", err)
	}
}

func (h *OnboardingHandler) clearForm(w http.ResponseWriter, r *http.Request) {
	sess, err := h.session.Get(r, onboardingSessionName)
	if err != nil {
		return
	}
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
}

func (h *OnboardingHandler) timezonesWithSelected(selected string) []timezoneOption {
	opts := make([]timezoneOption, len(commonTimezones))
	for i, tz := range commonTimezones {
		opts[i] = tz
		opts[i].Selected = tz.Value == selected
	}
	return opts
}

func stringOr(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func RequireOnboarding(appConfigRepo appconfig.Repository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		complete, err := appConfigRepo.Get(r.Context(), appconfig.KeyOnboardingComplete)
		if err != nil || complete != "true" {
			http.Redirect(w, r, "/onboard", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RedirectIfOnboarded(appConfigRepo appconfig.Repository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		complete, err := appConfigRepo.Get(r.Context(), appconfig.KeyOnboardingComplete)
		if err == nil && complete == "true" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func IsOnboarded(ctx context.Context, appConfigRepo appconfig.Repository) bool {
	complete, err := appConfigRepo.Get(ctx, appconfig.KeyOnboardingComplete)
	return err == nil && complete == "true"
}
