package sync

import "context"

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

type Result struct {
	Created      int
	Updated      int
	Deactivated  int
	RolesAdded   int
	RolesRemoved int
}

type Client interface {
	FetchRoster(ctx context.Context, memberType MemberType) ([]Member, error)
}
