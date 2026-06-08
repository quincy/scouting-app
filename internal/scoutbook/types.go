package scoutbook

import "encoding/json"

type MemberType string

const (
	EndpointUnitAdults MemberType = "adults"
	EndpointUnitYouths MemberType = "youths"
)

type positionItem struct {
	Name string `json:"position"`
}

type RosterMember struct {
	MemberID    string `json:"-"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	NickName    string `json:"nickName"`
	Gender      string `json:"gender"`
	PersonGUID  string `json:"personGuid"`
	Email       string `json:"email"`
	HomePhone   string `json:"homePhone"`
	MobilePhone string `json:"mobilePhone"`
	BirthDate   string `json:"dateOfBirth"`
	IsAdult     bool   `json:"isAdult"`
	Positions   string `json:"-"`
}

func (m *RosterMember) UnmarshalJSON(data []byte) error {
	type raw struct {
		MemberID    json.Number    `json:"memberId"`
		FirstName   string         `json:"firstName"`
		LastName    string         `json:"lastName"`
		NickName    string         `json:"nickName"`
		Gender      string         `json:"gender"`
		PersonGUID  string         `json:"personGuid"`
		Email       string         `json:"email"`
		HomePhone   string         `json:"homePhone"`
		MobilePhone string         `json:"mobilePhone"`
		BirthDate   string         `json:"dateOfBirth"`
		IsAdult     bool           `json:"isAdult"`
		Positions   []positionItem `json:"positions"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	m.MemberID = r.MemberID.String()
	m.FirstName = r.FirstName
	m.LastName = r.LastName
	m.NickName = r.NickName
	m.Gender = r.Gender
	m.PersonGUID = r.PersonGUID
	m.Email = r.Email
	m.HomePhone = r.HomePhone
	m.MobilePhone = r.MobilePhone
	m.BirthDate = r.BirthDate
	m.IsAdult = r.IsAdult
	parts := make([]string, 0, len(r.Positions))
	for _, p := range r.Positions {
		if p.Name != "" {
			parts = append(parts, p.Name)
		}
	}
	m.Positions = joinStrings(parts, ", ")
	return nil
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for _, v := range s[1:] {
		result += sep + v
	}
	return result
}

type unitRosterResponse struct {
	Users []RosterMember `json:"users"`
}
