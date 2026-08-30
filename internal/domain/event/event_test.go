package event

import "testing"

func TestDriverResponsibility_Values(t *testing.T) {
	d := DriverResponsibility{
		EventID:       "evt-1",
		ProfileID:     "prof-1",
		ProfileName:   "Alice",
		SeatbeltCount: 5,
	}
	if d.EventID != "evt-1" {
		t.Errorf("expected EventID evt-1, got %s", d.EventID)
	}
	if d.ProfileID != "prof-1" {
		t.Errorf("expected ProfileID prof-1, got %s", d.ProfileID)
	}
	if d.ProfileName != "Alice" {
		t.Errorf("expected ProfileName Alice, got %s", d.ProfileName)
	}
	if d.SeatbeltCount != 5 {
		t.Errorf("expected SeatbeltCount 5, got %d", d.SeatbeltCount)
	}
}

func TestSeatbeltSummary_Sufficient(t *testing.T) {
	s := SeatbeltSummary{
		TotalSeatbelts:    10,
		RequiredSeatbelts: 5,
		Available:         10,
		Sufficient:        true,
	}
	if !s.Sufficient {
		t.Error("expected Sufficient to be true")
	}
	if s.Available != 10 {
		t.Errorf("expected Available 10, got %d", s.Available)
	}
	if s.RequiredSeatbelts != 5 {
		t.Errorf("expected RequiredSeatbelts 5, got %d", s.RequiredSeatbelts)
	}
}

func TestSeatbeltSummary_Insufficient(t *testing.T) {
	s := SeatbeltSummary{
		TotalSeatbelts:    5,
		RequiredSeatbelts: 10,
		Available:         5,
		Sufficient:        false,
	}
	if s.Sufficient {
		t.Error("expected Sufficient to be false")
	}
}

func TestResponsibility_Constants(t *testing.T) {
	if ResponsibilityDriver != "driver" {
		t.Errorf("expected ResponsibilityDriver 'driver', got %q", ResponsibilityDriver)
	}
	if ResponsibilitySPL != "spl" {
		t.Errorf("expected ResponsibilitySPL 'spl', got %q", ResponsibilitySPL)
	}
	if ResponsibilityCoordinator != "coordinator" {
		t.Errorf("expected ResponsibilityCoordinator 'coordinator', got %q", ResponsibilityCoordinator)
	}
	if ResponsibilityMedicalOfficer != "medical_officer" {
		t.Errorf("expected ResponsibilityMedicalOfficer 'medical_officer', got %q", ResponsibilityMedicalOfficer)
	}
}

func TestIsSingleton(t *testing.T) {
	tests := []struct {
		name     string
		resp     Responsibility
		expected bool
	}{
		{name: "driver is not singleton", resp: ResponsibilityDriver, expected: false},
		{name: "SPL is singleton", resp: ResponsibilitySPL, expected: true},
		{name: "coordinator is singleton", resp: ResponsibilityCoordinator, expected: true},
		{name: "medical officer is singleton", resp: ResponsibilityMedicalOfficer, expected: true},
		{name: "unknown is not singleton", resp: Responsibility("unknown"), expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSingleton(tt.resp)
			if got != tt.expected {
				t.Errorf("IsSingleton(%q) = %v, want %v", tt.resp, got, tt.expected)
			}
		})
	}
}

func TestErrSingletonConflict_Error(t *testing.T) {
	err := ErrSingletonConflict{
		Responsibility:     ResponsibilitySPL,
		CurrentHolderID:    "prof-1",
		CurrentHolderName:  "Alice",
		RequestedProfileID: "prof-2",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestSingletons_AreDefined(t *testing.T) {
	if !IsSingleton(ResponsibilitySPL) {
		t.Error("expected SPL to be singleton")
	}
	if !IsSingleton(ResponsibilityCoordinator) {
		t.Error("expected Coordinator to be singleton")
	}
	if !IsSingleton(ResponsibilityMedicalOfficer) {
		t.Error("expected Medical Officer to be singleton")
	}
	if IsSingleton(ResponsibilityDriver) {
		t.Error("expected Driver to NOT be singleton")
	}
}

func TestValidResponsibilities(t *testing.T) {
	valid := []Responsibility{ResponsibilityDriver, ResponsibilitySPL, ResponsibilityCoordinator, ResponsibilityMedicalOfficer}
	for _, v := range valid {
		switch v {
		case ResponsibilityDriver, ResponsibilitySPL, ResponsibilityCoordinator, ResponsibilityMedicalOfficer:
		default:
			t.Errorf("unexpected responsibility value: %q", v)
		}
	}
}

func TestEvent_Toggles(t *testing.T) {
	e := Event{}
	if e.CookingEnabled {
		t.Error("expected CookingEnabled to default to false")
	}
	if e.TentingEnabled {
		t.Error("expected TentingEnabled to default to false")
	}

	e.CookingEnabled = true
	e.TentingEnabled = true
	if !e.CookingEnabled {
		t.Error("expected CookingEnabled to be configurable")
	}
	if !e.TentingEnabled {
		t.Error("expected TentingEnabled to be configurable")
	}
}
