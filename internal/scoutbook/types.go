package scoutbook

type MemberType string

const (
	EndpointAdults MemberType = "orgAdults"
	EndpointYouths MemberType = "orgYouths"
)

type RosterMember struct {
	MemberID   string `json:"memberId"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	PersonGUID string `json:"personGuid"`
}

type rosterResponse struct {
	Data []RosterMember `json:"members"`
}

type PersonProfile struct {
	Email        string `json:"email"`
	PrimaryPhone string `json:"primaryPhone"`
	BirthDate    string `json:"birthDate"`
}
