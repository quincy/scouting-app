package profile

import "time"

type MemberType string

const (
	MemberTypeAdult MemberType = "adult"
	MemberTypeYouth MemberType = "youth"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

type Profile struct {
	ID         string
	BSAID      string
	FirstName  string
	LastName   string
	Nickname   string
	Gender     string
	Email      string
	Phone      string
	Birthdate  time.Time
	MemberType MemberType
	Status     Status
	UserID     *string
	Positions  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
