package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

type Service struct {
	profiles profile.Repository
	rbac     rbac.Repository
	client   Client
}

func NewService(profiles profile.Repository, rbac rbac.Repository, client Client) *Service {
	return &Service{
		profiles: profiles,
		rbac:     rbac,
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
		log.Printf("[sync]   deduped: memberId=%s name=%s %s personGuid=%s email=%s phone=%s birth=%s",
			m.MemberID, m.FirstName, m.LastName, m.PersonGUID, m.Email, m.Phone, m.BirthDate)
	}

	activeBSAIDs := make(map[string]bool, len(members))

	result := &Result{}

	for _, m := range members {
		activeBSAIDs[m.MemberID] = true

		existing, err := s.profiles.GetByBSAID(ctx, m.MemberID)
		if err != nil {
			log.Printf("[sync] CREATE memberId=%s name=%s %s", m.MemberID, m.FirstName, m.LastName)
			p := &profile.Profile{
				BSAID:      m.MemberID,
				FirstName:  m.FirstName,
				LastName:   m.LastName,
				Nickname:   m.Nickname,
				Gender:     m.Gender,
				Positions:  m.Positions,
				MemberType: memberType(m.MemberID, adults, youths),
				Status:     profile.StatusActive,
			}

			applyRosterData(m, p)

			if err := s.profiles.Create(ctx, p); err != nil {
				return nil, fmt.Errorf("create profile %s: %w", m.MemberID, err)
			}
			result.Created++
			added, removed, err := s.reconcileRoles(ctx, p.ID, p.UserID, p.Positions)
			if err != nil {
				return nil, fmt.Errorf("reconcile roles for %s: %w", m.MemberID, err)
			}
			result.RolesAdded += added
			result.RolesRemoved += removed
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
			if existing.Nickname != m.Nickname {
				log.Printf("[sync] UPDATE memberId=%s nickname %q -> %q", m.MemberID, existing.Nickname, m.Nickname)
				existing.Nickname = m.Nickname
				updated = true
			}
			if existing.Gender != m.Gender {
				log.Printf("[sync] UPDATE memberId=%s gender %q -> %q", m.MemberID, existing.Gender, m.Gender)
				existing.Gender = m.Gender
				updated = true
			}
			if existing.Positions != m.Positions {
				log.Printf("[sync] UPDATE memberId=%s positions %q -> %q", m.MemberID, existing.Positions, m.Positions)
				existing.Positions = m.Positions
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

			if m.Email != "" && m.Email != existing.Email {
				log.Printf("[sync] UPDATE memberId=%s email %q -> %q", m.MemberID, existing.Email, m.Email)
				existing.Email = m.Email
				updated = true
			}
			if m.Phone != "" && m.Phone != existing.Phone {
				log.Printf("[sync] UPDATE memberId=%s phone %q -> %q", m.MemberID, existing.Phone, m.Phone)
				existing.Phone = m.Phone
				updated = true
			}
			if m.BirthDate != "" {
				if parsed, err := time.Parse("2006-01-02", m.BirthDate); err == nil {
					if !parsed.Equal(existing.Birthdate) {
						log.Printf("[sync] UPDATE memberId=%s birthdate %q -> %q", m.MemberID, existing.Birthdate.Format("2006-01-02"), m.BirthDate)
						existing.Birthdate = parsed
						updated = true
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

			added, removed, err := s.reconcileRoles(ctx, existing.ID, existing.UserID, existing.Positions)
			if err != nil {
				return nil, fmt.Errorf("reconcile roles for %s: %w", m.MemberID, err)
			}
			result.RolesAdded += added
			result.RolesRemoved += removed
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

	log.Printf("[sync] Result: created=%d updated=%d deactivated=%d rolesAdded=%d rolesRemoved=%d", result.Created, result.Updated, result.Deactivated, result.RolesAdded, result.RolesRemoved)
	return result, nil
}

func deduplicate(adults, youths []Member) []Member {
	seen := make(map[string]*Member, len(adults)+len(youths))

	for i := range adults {
		m := adults[i]
		seen[m.MemberID] = &Member{
			MemberID:   m.MemberID,
			FirstName:  m.FirstName,
			LastName:   m.LastName,
			Nickname:   m.Nickname,
			Gender:     m.Gender,
			PersonGUID: m.PersonGUID,
			Email:      m.Email,
			Phone:      m.Phone,
			BirthDate:  m.BirthDate,
			Positions:  m.Positions,
		}
	}

	for i := range youths {
		m := youths[i]
		if _, exists := seen[m.MemberID]; !exists {
			seen[m.MemberID] = &Member{
				MemberID:   m.MemberID,
				FirstName:  m.FirstName,
				LastName:   m.LastName,
				Nickname:   m.Nickname,
				Gender:     m.Gender,
				PersonGUID: m.PersonGUID,
				Email:      m.Email,
				Phone:      m.Phone,
				BirthDate:  m.BirthDate,
				Positions:  m.Positions,
			}
		}
	}

	result := make([]Member, 0, len(seen))
	for _, m := range seen {
		result = append(result, *m)
	}
	return result
}

func (s *Service) reconcileRoles(ctx context.Context, profileID string, userID *string, positions string) (added, removed int, err error) {
	if userID == nil {
		return 0, 0, nil
	}

	currentRoles, err := s.rbac.GetUserRoles(ctx, *userID)
	if err != nil {
		return 0, 0, fmt.Errorf("get user roles: %w", err)
	}

	currentRoleNames := make(map[string]bool)
	for _, role := range currentRoles {
		if role.Name != "parent" && role.Name != "admin" {
			currentRoleNames[role.Name] = true
		}
	}

	targetPositions := make(map[string]bool)
	if positions != "" {
		for _, pos := range strings.Split(positions, ", ") {
			targetPositions[pos] = true
		}
	}

	for pos := range targetPositions {
		if !currentRoleNames[pos] {
			role, err := s.rbac.GetRoleByName(ctx, pos)
			if err != nil {
				role = &rbac.Role{Name: pos}
				if err := s.rbac.CreateRole(ctx, role); err != nil {
					return 0, 0, fmt.Errorf("create role %q: %w", pos, err)
				}
			}
			if err := s.rbac.AssignRoleToUser(ctx, *userID, role.ID); err != nil {
				return 0, 0, fmt.Errorf("assign role %q: %w", pos, err)
			}
			log.Printf("[sync] ROLE ADDED memberId=%s role=%s", profileID, pos)
			added++
		}
	}

	for roleName := range currentRoleNames {
		if !targetPositions[roleName] {
			role, err := s.rbac.GetRoleByName(ctx, roleName)
			if err != nil {
				continue
			}
			if err := s.rbac.RemoveRoleFromUser(ctx, *userID, role.ID); err != nil {
				return 0, 0, fmt.Errorf("remove role %q: %w", roleName, err)
			}
			log.Printf("[sync] ROLE REMOVED memberId=%s role=%s", profileID, roleName)
			removed++
		}
	}

	return added, removed, nil
}

func memberType(bsaID string, adults, youths []Member) profile.MemberType {
	for _, a := range adults {
		if a.MemberID == bsaID {
			return profile.MemberTypeAdult
		}
	}
	return profile.MemberTypeYouth
}

func applyRosterData(m Member, p *profile.Profile) {
	if m.Email != "" {
		p.Email = m.Email
	}
	if m.Phone != "" {
		p.Phone = m.Phone
	}
	if m.BirthDate != "" {
		if parsed, err := time.Parse("2006-01-02", m.BirthDate); err == nil {
			p.Birthdate = parsed
		}
	}
}
