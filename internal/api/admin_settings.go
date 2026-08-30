package api

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/email"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

type adminSettingsPageData struct {
	Title         string
	UnitType      string
	UnitNumber    string
	OrgGUID       string
	Timezone      string
	Timezones     []timezoneOption
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPFrom      string
	MaxTentAgeGap string
	FlashSuccess  string
	Error         string
	AdminPerms
}

type SettingsHandler struct {
	appConfigRepo appconfig.Repository
	emailSvc      email.Service
	auth          *auth.AuthService
	profileRepo   profile.Repository
	rbacRepo      rbac.Repository
	tmpl          *template.Template
}

func NewSettingsHandler(appConfigRepo appconfig.Repository, emailSvc email.Service, authSvc *auth.AuthService, profileRepo profile.Repository, rbacRepo rbac.Repository) *SettingsHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &SettingsHandler{
		appConfigRepo: appConfigRepo,
		emailSvc:      emailSvc,
		auth:          authSvc,
		profileRepo:   profileRepo,
		rbacRepo:      rbacRepo,
		tmpl:          tmpl,
	}
}

func (h *SettingsHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	unitType := appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "UNIT_TYPE", appconfig.KeyUnitType, "Troop")
	unitNumber := appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "UNIT_NUMBER", appconfig.KeyUnitNumber, "")
	orgGUID, _ := h.appConfigRepo.Get(ctx, appconfig.KeyScoutbookOrgGUID)
	timezone := appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "DEFAULT_TIMEZONE", appconfig.KeyDefaultTimezone, "America/New_York")

	smtpHost, _ := h.appConfigRepo.Get(ctx, appconfig.KeySMTPHost)
	smtpPort, _ := h.appConfigRepo.Get(ctx, appconfig.KeySMTPPort)
	smtpUser, _ := h.appConfigRepo.Get(ctx, appconfig.KeySMTPUser)
	smtpFrom, _ := h.appConfigRepo.Get(ctx, appconfig.KeySMTPFrom)
	maxTentAgeGap := appconfig.GetWithHierarchy(ctx, h.appConfigRepo, "MAX_TENT_AGE_GAP", appconfig.KeyMaxTentAgeGap, "2")

	data := adminSettingsPageData{
		Title:         "Admin: Settings",
		UnitType:      unitType,
		UnitNumber:    unitNumber,
		OrgGUID:       orgGUID,
		Timezone:      timezone,
		Timezones:     timezonesWithSelected(timezone),
		SMTPHost:      smtpHost,
		SMTPPort:      smtpPort,
		SMTPUser:      smtpUser,
		SMTPFrom:      smtpFrom,
		MaxTentAgeGap: maxTentAgeGap,
	}

	if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
		data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") != "" {
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_settings", data); err != nil {
			log.Printf("admin_settings template: %v", err)
		}
		return
	}
	renderAdminLayout(w, h.tmpl, "admin_settings", data)
}

func (h *SettingsHandler) SettingsSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	unitType := r.FormValue("unit_type")
	unitNumber := r.FormValue("unit_number")
	orgGUID := r.FormValue("org_guid")
	timezone := r.FormValue("timezone")
	smtpHost := r.FormValue("smtp_host")
	smtpPort := r.FormValue("smtp_port")
	smtpUser := r.FormValue("smtp_user")
	smtpPass := r.FormValue("smtp_pass")
	smtpFrom := r.FormValue("smtp_from")
	maxTentAgeGap := r.FormValue("max_tent_age_gap")

	if unitType == "" {
		unitType = "Troop"
	}
	if timezone == "" {
		timezone = "America/New_York"
	}
	if maxTentAgeGap == "" {
		maxTentAgeGap = "2"
	}
	if maxAgeGap, err := strconv.Atoi(maxTentAgeGap); err != nil || maxAgeGap < 0 {
		data := adminSettingsPageData{
			Title:         "Admin: Settings",
			UnitType:      unitType,
			UnitNumber:    unitNumber,
			OrgGUID:       orgGUID,
			Timezone:      timezone,
			Timezones:     timezonesWithSelected(timezone),
			SMTPHost:      smtpHost,
			SMTPPort:      smtpPort,
			SMTPUser:      smtpUser,
			SMTPFrom:      smtpFrom,
			MaxTentAgeGap: maxTentAgeGap,
			Error:         "Max Tent Age Gap must be a whole number of years (0 or more).",
		}
		if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
			data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(h.tmpl.Clone())
		if err := t.ExecuteTemplate(w, "admin_settings", data); err != nil {
			log.Printf("admin_settings template: %v", err)
		}
		return
	}

	if err := h.appConfigRepo.Set(ctx, appconfig.KeyUnitType, unitType); err != nil {
		log.Printf("save unit type: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyUnitNumber, unitNumber); err != nil {
		log.Printf("save unit number: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyScoutbookOrgGUID, orgGUID); err != nil {
		log.Printf("save org guid: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyDefaultTimezone, timezone); err != nil {
		log.Printf("save timezone: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeySMTPHost, smtpHost); err != nil {
		log.Printf("save smtp host: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeySMTPPort, smtpPort); err != nil {
		log.Printf("save smtp port: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeySMTPUser, smtpUser); err != nil {
		log.Printf("save smtp user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if smtpPass != "" {
		if err := h.appConfigRepo.Set(ctx, appconfig.KeySMTPPass, smtpPass); err != nil {
			log.Printf("save smtp pass: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeySMTPFrom, smtpFrom); err != nil {
		log.Printf("save smtp from: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.appConfigRepo.Set(ctx, appconfig.KeyMaxTentAgeGap, maxTentAgeGap); err != nil {
		log.Printf("save max tent age gap: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := adminSettingsPageData{
		Title:         "Admin: Settings",
		UnitType:      unitType,
		UnitNumber:    unitNumber,
		OrgGUID:       orgGUID,
		Timezone:      timezone,
		Timezones:     timezonesWithSelected(timezone),
		SMTPHost:      smtpHost,
		SMTPPort:      smtpPort,
		SMTPUser:      smtpUser,
		SMTPFrom:      smtpFrom,
		MaxTentAgeGap: maxTentAgeGap,
		FlashSuccess:  "Settings saved successfully!",
	}

	if user, err := h.auth.GetAuthenticatedUser(r); err == nil && user != nil {
		data.AdminPerms = computeAdminPerms(r.Context(), h.rbacRepo, user.ID)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_settings", data); err != nil {
		log.Printf("admin_settings template: %v", err)
	}
}

func (h *SettingsHandler) TestEmail(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prof, err := h.profileRepo.GetByUserID(r.Context(), user.ID)
	if err != nil || prof == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	err = h.emailSvc.SendAdminNotification(ctx, []string{prof.Email}, "Test Email from Scout Events", "This is a test email to verify your SMTP configuration.")

	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		t := template.Must(h.tmpl.Clone())
		if err2 := t.ExecuteTemplate(w, "admin_settings_test_email_result", map[string]string{"Error": "Failed to send test email: " + err.Error()}); err2 != nil {
			log.Printf("test email error template: %v", err2)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_settings_test_email_result", map[string]string{"Success": "Test email sent to " + prof.Email + "!"}); err != nil {
		log.Printf("test email success template: %v", err)
	}
}

func timezonesWithSelected(selected string) []timezoneOption {
	opts := make([]timezoneOption, len(commonTimezones))
	for i, tz := range commonTimezones {
		opts[i] = tz
		opts[i].Selected = tz.Value == selected
	}
	return opts
}
