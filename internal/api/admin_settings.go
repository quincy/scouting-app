package api

import (
	"html/template"
	"log"
	"net/http"

	"scout-app/internal/domain/appconfig"
)

type adminSettingsPageData struct {
	Title        string
	UnitType     string
	UnitNumber   string
	OrgGUID      string
	Timezone     string
	Timezones    []timezoneOption
	FlashSuccess string
	Error        string
}

type SettingsHandler struct {
	appConfigRepo appconfig.Repository
	tmpl          *template.Template
}

func NewSettingsHandler(appConfigRepo appconfig.Repository) *SettingsHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/*.html"),
	)
	return &SettingsHandler{
		appConfigRepo: appConfigRepo,
		tmpl:          tmpl,
	}
}

func (h *SettingsHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	unitType, _ := h.appConfigRepo.Get(ctx, appconfig.KeyUnitType)
	if unitType == "" {
		unitType = "Troop"
	}

	unitNumber, _ := h.appConfigRepo.Get(ctx, appconfig.KeyUnitNumber)

	orgGUID, _ := h.appConfigRepo.Get(ctx, appconfig.KeyScoutbookOrgGUID)

	timezone, _ := h.appConfigRepo.Get(ctx, appconfig.KeyDefaultTimezone)
	if timezone == "" {
		timezone = "America/New_York"
	}

	data := adminSettingsPageData{
		Title:      "Admin: Settings",
		UnitType:   unitType,
		UnitNumber: unitNumber,
		OrgGUID:    orgGUID,
		Timezone:   timezone,
		Timezones:  timezonesWithSelected(timezone),
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

	if unitType == "" {
		unitType = "Troop"
	}
	if timezone == "" {
		timezone = "America/New_York"
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

	data := adminSettingsPageData{
		Title:        "Admin: Settings",
		UnitType:     unitType,
		UnitNumber:   unitNumber,
		OrgGUID:      orgGUID,
		Timezone:     timezone,
		Timezones:    timezonesWithSelected(timezone),
		FlashSuccess: "Settings saved successfully!",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.Must(h.tmpl.Clone())
	if err := t.ExecuteTemplate(w, "admin_settings", data); err != nil {
		log.Printf("admin_settings template: %v", err)
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
