package event

import (
	"fmt"
	"time"
)

type ListItem struct {
	ID            string
	Title         string
	Location      string
	StartTime     time.Time
	EndTime       time.Time
	Type          string
	AttendeeCount int
}

type Event struct {
	ID          string
	Title       string
	Description string
	Location    string
	StartTime   time.Time
	EndTime     time.Time
	CostCents   int
	CostDecimal float64
	Type        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Responsibility string

const (
	ResponsibilityDriver         Responsibility = "driver"
	ResponsibilitySPL            Responsibility = "spl"
	ResponsibilityCoordinator    Responsibility = "coordinator"
	ResponsibilityMedicalOfficer Responsibility = "medical_officer"
)

var singletonResponsibilities = map[Responsibility]bool{
	ResponsibilitySPL:            true,
	ResponsibilityCoordinator:    true,
	ResponsibilityMedicalOfficer: true,
}

func IsSingleton(r Responsibility) bool {
	return singletonResponsibilities[r]
}

type ErrSingletonConflict struct {
	Responsibility     Responsibility
	CurrentHolderID    string
	CurrentHolderName  string
	RequestedProfileID string
}

func (e ErrSingletonConflict) Error() string {
	return fmt.Sprintf("%q is a singleton responsibility; currently held by %s (%s)",
		e.Responsibility, e.CurrentHolderName, e.CurrentHolderID)
}

type ResponsibilityAssignment struct {
	EventID        string
	ProfileID      string
	ProfileName    string
	Responsibility Responsibility
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ResponsibilityHolder struct {
	ProfileID   string
	ProfileName string
}

type DriverResponsibility struct {
	EventID       string
	ProfileID     string
	ProfileName   string
	SeatbeltCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SeatbeltSummary struct {
	TotalSeatbelts    int
	RequiredSeatbelts int
	Available         int
	Sufficient        bool
}
