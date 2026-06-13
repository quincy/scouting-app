package parentyouthlink

import "time"

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusRevoked  Status = "revoked"
)

type ParentYouthConnection struct {
	ID              string
	ParentProfileID string
	YouthProfileID  string
	Status          Status
	RequestedAt     time.Time
	ApprovedAt      *time.Time
	ApprovedBy      *string
	CreatedAt       time.Time
}
