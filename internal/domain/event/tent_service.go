package event

import (
	"context"
	"errors"

	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
)

// ErrAdultInTent is returned when an adult attendee is placed into a tent.
// The app tracks youth tenting only; adult sleeping arrangements are outside
// the system.
var ErrAdultInTent = errors.New("adults cannot be placed into a tent")

// TentService coordinates tent assignment for an event. It applies the tenting
// rules (ValidateTent) to prospective placements so callers can either block a
// violation or explicitly override it.
type TentService struct {
	repo     Repository
	profiles profile.Repository
	links    parentyouthlink.Repository
}

func NewTentService(repo Repository, profiles profile.Repository, links parentyouthlink.Repository) *TentService {
	return &TentService{repo: repo, profiles: profiles, links: links}
}

// AfterWithdraw removes an attendee from their tent. It is safe to call for
// attendees who are not in any tent.
func (s *TentService) AfterWithdraw(ctx context.Context, eventID, profileID string) error {
	return s.repo.RemoveTentMember(ctx, eventID, profileID)
}

func (s *TentService) CreateTent(ctx context.Context, eventID string) (*Tent, error) {
	return s.repo.CreateTent(ctx, eventID)
}

func (s *TentService) DeleteTent(ctx context.Context, tentID string) error {
	return s.repo.DeleteTent(ctx, tentID)
}

func (s *TentService) ListTents(ctx context.Context, eventID string) ([]*Tent, error) {
	return s.repo.ListTents(ctx, eventID)
}

func (s *TentService) RemoveMember(ctx context.Context, eventID, profileID string) error {
	return s.repo.RemoveTentMember(ctx, eventID, profileID)
}

// AssignMember places a youth attendee into a tent after applying the strict
// tenting rules (mixed gender and age gap) to the prospective arrangement. The
// "never alone" rule is treated as a non-blocking warning, so a single scout
// may be placed and is flagged in the UI.
//
// The prospective tent is the target tent's current members plus the scout.
// When the arrangement violates a strict rule, no change is made and the
// violations are returned. An empty violations slice or a nil error means the
// placement was allowed.
func (s *TentService) AssignMember(ctx context.Context, eventID, tentID, profileID string, maxAgeGap int) ([]Violation, error) {
	p, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if p.MemberType != profile.MemberTypeYouth {
		return nil, ErrAdultInTent
	}

	tents, err := s.repo.ListTents(ctx, eventID)
	if err != nil {
		return nil, err
	}
	var target *Tent
	for _, t := range tents {
		if t.ID != tentID {
			continue
		}
		target = t
		for _, m := range t.Members {
			if m.ProfileID == profileID {
				return nil, nil
			}
		}
	}
	if target == nil {
		return nil, errors.New("tent not found")
	}

	evt, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	scouts, err := s.prospectiveScouts(ctx, target.Members, p)
	if err != nil {
		return nil, err
	}

	siblings, err := s.siblingPairs(ctx)
	if err != nil {
		return nil, err
	}

	violations := BlockingViolations(scouts, evt.StartTime, maxAgeGap, siblings)
	if len(violations) > 0 {
		return violations, nil
	}

	if err := s.repo.AssignTentMember(ctx, eventID, tentID, profileID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidTargetTents returns the tents a scout could be assigned to without
// violating a strict tenting rule (mixed gender or age gap). The scout's
// current tent is excluded via excludeTentID. The "never alone" rule is
// non-blocking, so a tent that would leave the scout alone is still returned —
// the UI flags it separately. An empty result means no tent is a valid option
// for this scout.
func (s *TentService) ValidTargetTents(ctx context.Context, eventID, profileID, excludeTentID string, maxAgeGap int) ([]*Tent, error) {
	p, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}

	evt, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	siblings, err := s.siblingPairs(ctx)
	if err != nil {
		return nil, err
	}

	tents, err := s.repo.ListTents(ctx, eventID)
	if err != nil {
		return nil, err
	}

	var valid []*Tent
	for _, t := range tents {
		if t.ID == excludeTentID {
			continue
		}
		scouts, err := s.prospectiveScouts(ctx, t.Members, p)
		if err != nil {
			return nil, err
		}
		if len(BlockingViolations(scouts, evt.StartTime, maxAgeGap, siblings)) > 0 {
			continue
		}
		valid = append(valid, t)
	}
	return valid, nil
}

func (s *TentService) prospectiveScouts(ctx context.Context, members []TentMember, added *profile.Profile) ([]TentScout, error) {
	scouts := make([]TentScout, 0, len(members)+1)
	seen := make(map[string]bool, len(members)+1)
	for _, m := range members {
		p, err := s.profiles.GetByID(ctx, m.ProfileID)
		if err != nil {
			return nil, err
		}
		seen[p.ID] = true
		scouts = append(scouts, tentScoutFromProfile(p))
	}
	if !seen[added.ID] {
		scouts = append(scouts, tentScoutFromProfile(added))
	}
	return scouts, nil
}

func tentScoutFromProfile(p *profile.Profile) TentScout {
	return TentScout{
		ID:        p.ID,
		Name:      p.DisplayName(),
		Gender:    p.Gender,
		Birthdate: p.Birthdate,
	}
}

func (s *TentService) siblingPairs(ctx context.Context) (map[ScoutPair]bool, error) {
	links, err := s.links.ListByStatus(ctx, parentyouthlink.StatusApproved)
	if err != nil {
		return nil, err
	}
	approved := make([]ApprovedParentLink, 0, len(links))
	for _, l := range links {
		if l.Status != parentyouthlink.StatusApproved {
			continue
		}
		approved = append(approved, ApprovedParentLink{
			ParentProfileID: l.ParentProfileID,
			YouthProfileID:  l.YouthProfileID,
		})
	}
	return SiblingPairs(approved), nil
}
