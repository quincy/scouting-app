package sync

import (
	"context"
	"fmt"
	stdSync "sync"
	"time"

	"scout-app/internal/domain/profile"
)

const profileFetchConcurrency = 5

type Service struct {
	profiles profile.Repository
	client   Client
}

func NewService(profiles profile.Repository, client Client) *Service {
	return &Service{
		profiles: profiles,
		client:   client,
	}
}

func (s *Service) Sync(ctx context.Context) (*Result, error) {
	adults, err := s.client.FetchRoster(ctx, EndpointAdults)
	if err != nil {
		return nil, fmt.Errorf("fetch adults: %w", err)
	}

	youths, err := s.client.FetchRoster(ctx, EndpointYouths)
	if err != nil {
		return nil, fmt.Errorf("fetch youths: %w", err)
	}

	members := deduplicate(adults, youths)

	profilesByGUID := s.fetchAllProfiles(ctx, members)

	activeBSAIDs := make(map[string]bool, len(members))

	result := &Result{}

	for _, m := range members {
		activeBSAIDs[m.MemberID] = true

		sbProfile := profilesByGUID[m.PersonGUID]

		existing, err := s.profiles.GetByBSAID(ctx, m.MemberID)
		if err != nil {
			p := &profile.Profile{
				BSAID:      m.MemberID,
				FirstName:  m.FirstName,
				LastName:   m.LastName,
				MemberType: memberType(m.MemberID, adults, youths),
				Status:     profile.StatusActive,
			}

			applyPersonProfile(sbProfile, p)

			if err := s.profiles.Create(ctx, p); err != nil {
				return nil, fmt.Errorf("create profile %s: %w", m.MemberID, err)
			}
			result.Created++
		} else {
			updated := false
			if existing.FirstName != m.FirstName {
				existing.FirstName = m.FirstName
				updated = true
			}
			if existing.LastName != m.LastName {
				existing.LastName = m.LastName
				updated = true
			}

			mt := memberType(m.MemberID, adults, youths)
			if existing.MemberType != mt {
				existing.MemberType = mt
				updated = true
			}

			if existing.Status != profile.StatusActive {
				existing.Status = profile.StatusActive
				updated = true
			}

			if sbProfile != nil {
				if sbProfile.Email != "" && sbProfile.Email != existing.Email {
					existing.Email = sbProfile.Email
					updated = true
				}
				if sbProfile.PrimaryPhone != "" && sbProfile.PrimaryPhone != existing.Phone {
					existing.Phone = sbProfile.PrimaryPhone
					updated = true
				}
				if sbProfile.BirthDate != "" {
					if parsed, err := time.Parse("2006-01-02", sbProfile.BirthDate); err == nil {
						if !parsed.Equal(existing.Birthdate) {
							existing.Birthdate = parsed
							updated = true
						}
					}
				}
			}

			if updated {
				existing.UpdatedAt = time.Now()
				if err := s.profiles.Update(ctx, existing); err != nil {
					return nil, fmt.Errorf("update profile %s: %w", m.MemberID, err)
				}
				result.Updated++
			}
		}
	}

	allActive, err := s.profiles.ListByStatus(ctx, profile.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("list active profiles: %w", err)
	}
	for _, p := range allActive {
		if !activeBSAIDs[p.BSAID] {
			p.Status = profile.StatusInactive
			p.UpdatedAt = time.Now()
			if err := s.profiles.Update(ctx, p); err != nil {
				return nil, fmt.Errorf("deactivate profile %s: %w", p.BSAID, err)
			}
			result.Deactivated++
		}
	}

	return result, nil
}

func (s *Service) fetchAllProfiles(ctx context.Context, members []Member) map[string]*PersonProfile {
	type job struct {
		personGUID string
		result     *PersonProfile
	}

	jobs := make(chan job, len(members))
	sem := make(chan struct{}, profileFetchConcurrency)
	var wg stdSync.WaitGroup

	for _, m := range members {
		if m.PersonGUID == "" {
			continue
		}
		wg.Add(1)
		memberGUID := m.PersonGUID
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			p, err := s.client.FetchProfile(ctx, memberGUID)
			if err != nil {
				return
			}
			jobs <- job{personGUID: memberGUID, result: p}
		}()
	}

	go func() {
		wg.Wait()
		close(jobs)
	}()

	profilesByGUID := make(map[string]*PersonProfile, len(members))
	for j := range jobs {
		profilesByGUID[j.personGUID] = j.result
	}

	return profilesByGUID
}

func deduplicate(adults, youths []Member) []Member {
	seen := make(map[string]*Member, len(adults)+len(youths))

	for i := range adults {
		m := adults[i]
		seen[m.MemberID] = &Member{
			MemberID:   m.MemberID,
			FirstName:  m.FirstName,
			LastName:   m.LastName,
			PersonGUID: m.PersonGUID,
		}
	}

	for i := range youths {
		m := youths[i]
		if _, exists := seen[m.MemberID]; !exists {
			seen[m.MemberID] = &Member{
				MemberID:   m.MemberID,
				FirstName:  m.FirstName,
				LastName:   m.LastName,
				PersonGUID: m.PersonGUID,
			}
		}
	}

	result := make([]Member, 0, len(seen))
	for _, m := range seen {
		result = append(result, *m)
	}
	return result
}

func memberType(bsaID string, adults, youths []Member) profile.MemberType {
	for _, a := range adults {
		if a.MemberID == bsaID {
			return profile.MemberTypeAdult
		}
	}
	return profile.MemberTypeYouth
}

func applyPersonProfile(sbProfile *PersonProfile, p *profile.Profile) {
	if sbProfile == nil {
		return
	}
	if sbProfile.Email != "" {
		p.Email = sbProfile.Email
	}
	if sbProfile.PrimaryPhone != "" {
		p.Phone = sbProfile.PrimaryPhone
	}
	if sbProfile.BirthDate != "" {
		if parsed, err := time.Parse("2006-01-02", sbProfile.BirthDate); err == nil {
			p.Birthdate = parsed
		}
	}
}
