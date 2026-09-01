package event

import (
	"context"
	"errors"

	"scout-app/internal/domain/profile"
)

type CookingPatrolService struct {
	repo     Repository
	profiles profile.Repository
}

var ErrAdultInYouthPatrol = errors.New("adults cannot be placed into a youth cooking patrol")

func NewCookingPatrolService(repo Repository, profiles profile.Repository) *CookingPatrolService {
	return &CookingPatrolService{repo: repo, profiles: profiles}
}

func (s *CookingPatrolService) AfterSignUp(ctx context.Context, eventID, profileID string) error {
	evt, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if !evt.CookingEnabled {
		return nil
	}
	p, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p.MemberType != profile.MemberTypeAdult {
		return nil
	}

	patrol, err := s.adultPatrol(ctx, evt)
	if err != nil {
		return err
	}
	return s.repo.AssignCookingPatrolMember(ctx, evt.ID, patrol.ID, profileID)
}

func (s *CookingPatrolService) adultPatrol(ctx context.Context, evt *Event) (*CookingPatrol, error) {
	patrols, err := s.repo.ListCookingPatrols(ctx, evt.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range patrols {
		if p.IsAdult {
			return p, nil
		}
	}
	return s.repo.CreateCookingPatrol(ctx, evt.ID, true)
}

func (s *CookingPatrolService) AfterWithdraw(ctx context.Context, eventID, profileID string) error {
	patrols, err := s.repo.ListCookingPatrols(ctx, eventID)
	if err != nil {
		return err
	}
	for _, p := range patrols {
		for _, m := range p.Members {
			if m.ProfileID == profileID && m.IsCook {
				if err := s.repo.ClearCookingPatrolCook(ctx, p.ID); err != nil {
					return err
				}
			}
		}
	}
	return s.repo.RemoveCookingPatrolMember(ctx, eventID, profileID)
}

func (s *CookingPatrolService) AssignMember(ctx context.Context, eventID, patrolID, profileID string) error {
	p, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p.MemberType == profile.MemberTypeAdult {
		patrols, err := s.repo.ListCookingPatrols(ctx, eventID)
		if err != nil {
			return err
		}
		var adultPatrol bool
		for _, patrol := range patrols {
			if patrol.ID == patrolID {
				adultPatrol = patrol.IsAdult
				break
			}
		}
		if !adultPatrol {
			return ErrAdultInYouthPatrol
		}
	}
	return s.repo.AssignCookingPatrolMember(ctx, eventID, patrolID, profileID)
}

func (s *CookingPatrolService) SetCook(ctx context.Context, eventID, patrolID, profileID string) error {
	return s.repo.SetCookingPatrolCook(ctx, eventID, patrolID, profileID)
}

func (s *CookingPatrolService) ClearCook(ctx context.Context, eventID, patrolID string) error {
	return s.repo.ClearCookingPatrolCook(ctx, patrolID)
}

func (s *CookingPatrolService) ListPatrols(ctx context.Context, eventID string) ([]*CookingPatrol, error) {
	return s.repo.ListCookingPatrols(ctx, eventID)
}

func (s *CookingPatrolService) CreatePatrol(ctx context.Context, eventID string, isAdult bool) (*CookingPatrol, error) {
	return s.repo.CreateCookingPatrol(ctx, eventID, isAdult)
}

func (s *CookingPatrolService) DeletePatrol(ctx context.Context, patrolID string) error {
	return s.repo.DeleteCookingPatrol(ctx, patrolID)
}

func (s *CookingPatrolService) RemoveMember(ctx context.Context, eventID, profileID string) error {
	patrols, err := s.repo.ListCookingPatrols(ctx, eventID)
	if err != nil {
		return err
	}
	for _, p := range patrols {
		for _, m := range p.Members {
			if m.ProfileID == profileID && m.IsCook {
				if err := s.repo.ClearCookingPatrolCook(ctx, p.ID); err != nil {
					return err
				}
			}
		}
	}
	return s.repo.RemoveCookingPatrolMember(ctx, eventID, profileID)
}
