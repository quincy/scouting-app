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
		sbType = scoutbook.EndpointAdults
	case EndpointYouths:
		sbType = scoutbook.EndpointYouths
	default:
		return nil, nil
	}

	members, err := a.inner.FetchRoster(ctx, sbType)
	if err != nil {
		return nil, err
	}

	result := make([]Member, len(members))
	for i, m := range members {
		result[i] = Member{
			MemberID:   m.MemberID,
			FirstName:  m.FirstName,
			LastName:   m.LastName,
			PersonGUID: m.PersonGUID,
		}
	}
	return result, nil
}

func (a *scoutbookAdapter) FetchProfile(ctx context.Context, personGUID string) (*PersonProfile, error) {
	p, err := a.inner.FetchProfile(ctx, personGUID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return &PersonProfile{
		Email:        p.Email,
		PrimaryPhone: p.PrimaryPhone,
		BirthDate:    p.BirthDate,
	}, nil
}
