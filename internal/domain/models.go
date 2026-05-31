package domain

import "time"

// User represents a security principal in the system.
type User struct {
	ID           string // UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Role represents a designation assigned to a User.
type Role struct {
	ID   string // UUID
	Name string
}

// Permission represents a specific allowed action.
type Permission struct {
	ID   string // UUID
	Name string
}

// Event represents a planned troop activity or campout.
type Event struct {
	ID          string // UUID
	Title       string
	Description string
	Location    string
	StartTime   time.Time
	EndTime     time.Time
	CostCents   int
	CostDecimal float64 // Automatically computed database projection
	Type        string  // e.g. "campout"
	CreatedAt   time.Time
}
