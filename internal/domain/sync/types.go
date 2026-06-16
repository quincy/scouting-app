package sync

import (
	"context"
	"time"

	"scout-app/internal/domain/profile"
)

type MemberType string

const (
	EndpointAdults MemberType = "adults"
	EndpointYouths MemberType = "youths"
)

type Member struct {
	MemberID   string
	FirstName  string
	LastName   string
	Nickname   string
	Gender     string
	PersonGUID string
	Email      string
	Phone      string
	BirthDate  string
	Positions  string
}

type ProfileSnapshot struct {
	BSAID      string
	FirstName  string
	LastName   string
	Nickname   string
	Gender     string
	Email      string
	Phone      string
	Birthdate  time.Time
	MemberType profile.MemberType
	Status     profile.Status
	Positions  string
	UserID     *string
}

type ProfileReport struct {
	MemberID     string
	Name         string
	Status       string // "created" | "updated" | "deactivated"
	Old          *ProfileSnapshot
	New          ProfileSnapshot
	RolesAdded   []string
	RolesRemoved []string
}

type Result struct {
	Created      int
	Updated      int
	Deactivated  int
	RolesAdded   int
	RolesRemoved int
	Profiles     []ProfileReport
}

type Client interface {
	FetchRoster(ctx context.Context, memberType MemberType) ([]Member, error)
}
