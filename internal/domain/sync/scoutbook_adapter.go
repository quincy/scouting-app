package sync

import (
	"context"

	"scout-app/internal/scoutbook"
)

type scoutbookAdapter struct {
	inner *scoutbook.Client
}

func NewScoutbookClientAdapter(client *scoutbook.Client) Client {
	return &scoutbookAdapter{inner: client}
}

func (a *scoutbookAdapter) FetchRoster(ctx context.Context, memberType MemberType) ([]Member, error) {
	var sbType scoutbook.MemberType
	switch memberType {
	case EndpointAdults:
		sbType = scoutbook.EndpointUnitAdults
	case EndpointYouths:
		sbType = scoutbook.EndpointUnitYouths
	default:
		return nil, nil
	}

	members, err := a.inner.FetchRoster(ctx, sbType)
	if err != nil {
		return nil, err
	}

	result := make([]Member, len(members))
	for i, m := range members {
		phone := m.HomePhone
		if phone == "" {
			phone = m.MobilePhone
		}
		result[i] = Member{
			MemberID:   m.MemberID,
			FirstName:  m.FirstName,
			LastName:   m.LastName,
			Nickname:   m.NickName,
			Gender:     m.Gender,
			PersonGUID: m.PersonGUID,
			Email:      m.Email,
			Phone:      phone,
			BirthDate:  m.BirthDate,
			Positions:  m.Positions,
		}
	}
	return result, nil
}
