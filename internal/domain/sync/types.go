package sync

import "context"

type MemberType string

const (
	EndpointAdults MemberType = "orgAdults"
	EndpointYouths MemberType = "orgYouths"
)

type Member struct {
	MemberID   string
	FirstName  string
	LastName   string
	PersonGUID string
}

type PersonProfile struct {
	Email        string
	PrimaryPhone string
	BirthDate    string
}

type Result struct {
	Created     int
	Updated     int
	Deactivated int
}

type Client interface {
	FetchRoster(ctx context.Context, memberType MemberType) ([]Member, error)
	FetchProfile(ctx context.Context, personGUID string) (*PersonProfile, error)
}
