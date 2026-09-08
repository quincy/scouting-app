package api

import (
	"log"
	"net/http"
	"sort"
	"time"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
)

type rosterPageData struct {
	Event           *event.Event
	EventID         string
	IsPast          bool
	AdultAttendees  []rosterAttendeeVM
	YouthAttendees  []rosterAttendeeVM
	AdultCount      int
	YouthCount      int
	AttendeeCount   int
	CookingEnabled  bool
	TentingEnabled  bool
	DriversEnabled  bool
	Cooking         *cookingSectionData
	Tenting         *tentingSectionData
	Drivers         []event.DriverResponsibility
	SeatbeltSummary event.SeatbeltSummary
}

type rosterAttendeeVM struct {
	ProfileID        string
	ProfileName      string
	Age              int
	IsDriver         bool
	SeatbeltCount    int
	IsSPL            bool
	IsCoordinator    bool
	IsMedicalOfficer bool
}

func buildRosterAttendeeVMs(attendees []*profile.Profile, drivers []event.DriverResponsibility, responsibilities []event.ResponsibilityAssignment, startTime time.Time) (youthVMs, adultVMs []rosterAttendeeVM) {
	for _, p := range attendees {
		vm := rosterAttendeeVM{
			ProfileID:   p.ID,
			ProfileName: p.DisplayName(),
			Age:         event.AgeWholeYears(p.Birthdate, startTime),
		}
		if p.MemberType == profile.MemberTypeYouth {
			youthVMs = append(youthVMs, vm)
		} else {
			adultVMs = append(adultVMs, vm)
		}
	}
	enrichRosterVMsWithDrivers(youthVMs, drivers)
	enrichRosterVMsWithDrivers(adultVMs, drivers)
	enrichRosterVMsWithResponsibilities(youthVMs, responsibilities)
	enrichRosterVMsWithResponsibilities(adultVMs, responsibilities)
	sort.Slice(youthVMs, func(i, j int) bool {
		return youthVMs[i].ProfileName < youthVMs[j].ProfileName
	})
	sort.Slice(adultVMs, func(i, j int) bool {
		return adultVMs[i].ProfileName < adultVMs[j].ProfileName
	})
	return youthVMs, adultVMs
}

func enrichRosterVMsWithDrivers(vms []rosterAttendeeVM, drivers []event.DriverResponsibility) {
	for i := range vms {
		for _, d := range drivers {
			if vms[i].ProfileID != d.ProfileID {
				continue
			}
			vms[i].IsDriver = true
			vms[i].SeatbeltCount = d.SeatbeltCount
			break
		}
	}
}

func enrichRosterVMsWithResponsibilities(vms []rosterAttendeeVM, responsibilities []event.ResponsibilityAssignment) {
	for i := range vms {
		for _, r := range responsibilities {
			if vms[i].ProfileID != r.ProfileID {
				continue
			}
			switch r.Responsibility {
			case event.ResponsibilitySPL:
				vms[i].IsSPL = true
			case event.ResponsibilityCoordinator:
				vms[i].IsCoordinator = true
			case event.ResponsibilityMedicalOfficer:
				vms[i].IsMedicalOfficer = true
			}
		}
	}
}

func (h *EventHandler) RosterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.isAdmin(ctx, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	eventID := muxVars(r)["id"]
	if eventID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	evt, err := h.repo.GetByID(ctx, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	attendees, err := h.repo.GetAttendees(ctx, eventID)
	if err != nil {
		log.Printf("RosterPage GetAttendees: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	responsibilities, err := h.repo.GetResponsibilities(ctx, eventID)
	if err != nil {
		log.Printf("RosterPage GetResponsibilities: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	drivers, err := h.repo.GetDrivers(ctx, eventID)
	if err != nil {
		log.Printf("RosterPage GetDrivers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isPast := evt.EndTime.Before(time.Now())
	data := &rosterPageData{
		Event:           evt,
		EventID:         eventID,
		IsPast:          isPast,
		CookingEnabled:  evt.CookingEnabled,
		TentingEnabled:  evt.TentingEnabled,
		DriversEnabled:  evt.DriversEnabled,
		Drivers:         drivers,
		SeatbeltSummary: event.SeatbeltSummary{},
	}

	youthVMs, adultVMs := buildRosterAttendeeVMs(attendees, drivers, responsibilities, evt.StartTime)
	data.YouthAttendees = youthVMs
	data.AdultAttendees = adultVMs
	data.YouthCount = len(youthVMs)
	data.AdultCount = len(adultVMs)
	data.AttendeeCount = len(attendees)

	if evt.CookingEnabled {
		cookingData, err := h.buildCookingSectionData(ctx, eventID, isPast, false)
		if err != nil {
			log.Printf("RosterPage buildCookingSectionData: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		data.Cooking = cookingData
	}

	if evt.TentingEnabled {
		tentingData, err := h.buildTentingSectionData(ctx, eventID, evt.StartTime, isPast, false)
		if err != nil {
			log.Printf("RosterPage buildTentingSectionData: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		data.Tenting = tentingData
	}

	if evt.DriversEnabled {
		summary, err := h.repo.GetSeatbeltSummary(ctx, eventID)
		if err != nil {
			log.Printf("RosterPage GetSeatbeltSummary: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		data.SeatbeltSummary = *summary
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "roster.html", data); err != nil {
		log.Printf("template execution (roster.html): %v", err)
	}
}
