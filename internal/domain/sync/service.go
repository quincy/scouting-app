package sync

import (
	"context"
	"fmt"
	"log"
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
	log.Printf("[sync] Fetched %d adults", len(adults))

	youths, err := s.client.FetchRoster(ctx, EndpointYouths)
	if err != nil {
		return nil, fmt.Errorf("fetch youths: %w", err)
	}
	log.Printf("[sync] Fetched %d youths", len(youths))

	members := deduplicate(adults, youths)
	log.Printf("[sync] After dedup: %d unique members", len(members))
	for _, m := range members {
		log.Printf("[sync]   deduped: memberId=%s name=%s %s personGuid=%s",
			m.MemberID, m.FirstName, m.LastName, m.PersonGUID)
	}

	profilesByGUID := s.fetchAllProfiles(ctx, members)
	log.Printf("[sync] Fetched %d person profiles", len(profilesByGUID))
	for guid, p := range profilesByGUID {
		if p != nil {
			log.Printf("[sync]   profile: personGuid=%s email=%s phone=%s birth=%s",
				guid, p.Email, p.PrimaryPhone, p.BirthDate)
		} else {
			log.Printf("[sync]   profile: personGuid=%s (not found)", guid)
		}
	}

	activeBSAIDs := make(map[string]bool, len(members))

	result := &Result{}

	for _, m := range members {
		activeBSAIDs[m.MemberID] = true

		sbProfile := profilesByGUID[m.PersonGUID]

		existing, err := s.profiles.GetByBSAID(ctx, m.MemberID)
		if err != nil {
			log.Printf("[sync] CREATE memberId=%s name=%s %s", m.MemberID, m.FirstName, m.LastName)
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
				log.Printf("[sync] UPDATE memberId=%s firstName %q -> %q", m.MemberID, existing.FirstName, m.FirstName)
				existing.FirstName = m.FirstName
				updated = true
			}
			if existing.LastName != m.LastName {
				log.Printf("[sync] UPDATE memberId=%s lastName %q -> %q", m.MemberID, existing.LastName, m.LastName)
				existing.LastName = m.LastName
				updated = true
			}

			mt := memberType(m.MemberID, adults, youths)
			if existing.MemberType != mt {
				log.Printf("[sync] UPDATE memberId=%s memberType %q -> %q", m.MemberID, existing.MemberType, mt)
				existing.MemberType = mt
				updated = true
			}

			if existing.Status != profile.StatusActive {
				log.Printf("[sync] UPDATE memberId=%s status %q -> active", m.MemberID, existing.Status)
				existing.Status = profile.StatusActive
				updated = true
			}

			if sbProfile != nil {
				if sbProfile.Email != "" && sbProfile.Email != existing.Email {
					log.Printf("[sync] UPDATE memberId=%s email %q -> %q", m.MemberID, existing.Email, sbProfile.Email)
					existing.Email = sbProfile.Email
					updated = true
				}
				if sbProfile.PrimaryPhone != "" && sbProfile.PrimaryPhone != existing.Phone {
					log.Printf("[sync] UPDATE memberId=%s phone %q -> %q", m.MemberID, existing.Phone, sbProfile.PrimaryPhone)
					existing.Phone = sbProfile.PrimaryPhone
					updated = true
				}
				if sbProfile.BirthDate != "" {
					if parsed, err := time.Parse("2006-01-02", sbProfile.BirthDate); err == nil {
						if !parsed.Equal(existing.Birthdate) {
							log.Printf("[sync] UPDATE memberId=%s birthdate %q -> %q", m.MemberID, existing.Birthdate.Format("2006-01-02"), sbProfile.BirthDate)
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
				log.Printf("[sync] UPDATED memberId=%s", m.MemberID)
			} else {
				log.Printf("[sync] SKIP memberId=%s (no changes)", m.MemberID)
			}
		}
	}

	allActive, err := s.profiles.ListByStatus(ctx, profile.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("list active profiles: %w", err)
	}
	for _, p := range allActive {
		if !activeBSAIDs[p.BSAID] {
			log.Printf("[sync] DEACTIVATE memberId=%s name=%s %s (not in roster)", p.BSAID, p.FirstName, p.LastName)
			p.Status = profile.StatusInactive
			p.UpdatedAt = time.Now()
			if err := s.profiles.Update(ctx, p); err != nil {
				return nil, fmt.Errorf("deactivate profile %s: %w", p.BSAID, err)
			}
			result.Deactivated++
		}
	}

	log.Printf("[sync] Result: created=%d updated=%d deactivated=%d", result.Created, result.Updated, result.Deactivated)
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
