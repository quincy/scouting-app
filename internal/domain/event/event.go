package event

import "time"

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
