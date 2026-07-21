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
