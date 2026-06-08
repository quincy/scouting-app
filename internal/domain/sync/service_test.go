package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
)

type mockClient struct {
	adults []Member
	youths []Member
}

func (m *mockClient) FetchRoster(ctx context.Context, memberType MemberType) ([]Member, error) {
	if memberType == EndpointAdults {
		return m.adults, nil
	}
	return m.youths, nil
}

func TestSync_CreatesNewProfiles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", Nickname: "Johnny", Gender: "M", PersonGUID: "guid-1", Email: "john@example.com", Phone: "555-0100", BirthDate: "1990-01-15", Positions: "Scoutmaster, Troop Admin"},
		},
		youths: nil,
	}

	svc := NewService(repo, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Created != 1 {
		t.Errorf("expected 1 created, got %d", result.Created)
	}
	if result.Updated != 0 {
		t.Errorf("expected 0 updated, got %d", result.Updated)
	}
	if result.Deactivated != 0 {
		t.Errorf("expected 0 deactivated, got %d", result.Deactivated)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.FirstName != "John" || p.LastName != "Doe" {
		t.Errorf("unexpected name: %s %s", p.FirstName, p.LastName)
	}
	if p.Nickname != "Johnny" {
		t.Errorf("expected nickname Johnny, got %s", p.Nickname)
	}
	if p.Gender != "M" {
		t.Errorf("expected gender M, got %s", p.Gender)
	}
	if p.Email != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", p.Email)
	}
	if p.Phone != "555-0100" {
		t.Errorf("expected phone 555-0100, got %s", p.Phone)
	}
	if p.MemberType != profile.MemberTypeAdult {
		t.Errorf("expected adult, got %s", p.MemberType)
	}
	if p.Status != profile.StatusActive {
		t.Errorf("expected active, got %s", p.Status)
	}
	if p.Positions != "Scoutmaster, Troop Admin" {
		t.Errorf("expected positions, got %s", p.Positions)
	}
}

func TestSync_CreatesYouthProfiles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	client := &mockClient{
		adults: nil,
		youths: []Member{
			{MemberID: "200", FirstName: "Jimmy", LastName: "Jones", PersonGUID: "guid-2"},
		},
	}

	svc := NewService(repo, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Created != 1 {
		t.Fatalf("expected 1 created, got %d", result.Created)
	}

	p, err := repo.GetByBSAID(ctx, "200")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.MemberType != profile.MemberTypeYouth {
		t.Errorf("expected youth, got %s", p.MemberType)
	}
}

func TestSync_DedupAdult(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
		youths: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
	}

	svc := NewService(repo, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Created != 1 {
		t.Fatalf("expected 1 created (deduped), got %d", result.Created)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.MemberType != profile.MemberTypeAdult {
		t.Errorf("expected adult (appeared in both), got %s", p.MemberType)
	}
}

func TestSync_UpdatesExistingProfiles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()

	birthdate := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	existing := &profile.Profile{
		BSAID:      "100",
		FirstName:  "OldFirst",
		LastName:   "OldLast",
		Nickname:   "OldNick",
		Gender:     "F",
		Email:      "old@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		Birthdate:  birthdate,
		Positions:  "Old Position",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", Nickname: "Johnny", Gender: "M", PersonGUID: "guid-1", Email: "new@example.com", Phone: "555-9999", BirthDate: "1990-01-15", Positions: "Scoutmaster"},
		},
	}

	svc := NewService(repo, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Created != 0 {
		t.Errorf("expected 0 created, got %d", result.Created)
	}
	if result.Updated != 1 {
		t.Errorf("expected 1 updated, got %d", result.Updated)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.FirstName != "John" {
		t.Errorf("expected first name John, got %s", p.FirstName)
	}
	if p.Nickname != "Johnny" {
		t.Errorf("expected nickname Johnny, got %s", p.Nickname)
	}
	if p.Gender != "M" {
		t.Errorf("expected gender M, got %s", p.Gender)
	}
	if p.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", p.Email)
	}
	if p.Positions != "Scoutmaster" {
		t.Errorf("expected positions Scoutmaster, got %s", p.Positions)
	}
}

func TestSync_EmailSyncRule(t *testing.T) {
	ctx := t.Context()

	t.Run("scoutbook_email_overwrites_local", func(t *testing.T) {
		repo := newMockProfileRepository()
		existing := &profile.Profile{
			BSAID:      "100",
			FirstName:  "John",
			LastName:   "Doe",
			Email:      "old@example.com",
			MemberType: profile.MemberTypeAdult,
			Status:     profile.StatusActive,
		}
		if err := repo.Create(ctx, existing); err != nil {
			t.Fatalf("Create existing profile: %v", err)
		}

		client := &mockClient{
			adults: []Member{
				{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "new@example.com"},
			},
		}

		svc := NewService(repo, client)
		_, err := svc.Sync(ctx)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		p, _ := repo.GetByBSAID(ctx, "100")
		if p.Email != "new@example.com" {
			t.Errorf("expected new@example.com, got %s", p.Email)
		}
	})

	t.Run("local_preserved_when_scoutbook_has_no_email", func(t *testing.T) {
		repo := newMockProfileRepository()
		existing := &profile.Profile{
			BSAID:      "100",
			FirstName:  "John",
			LastName:   "Doe",
			Email:      "local@example.com",
			MemberType: profile.MemberTypeAdult,
			Status:     profile.StatusActive,
		}
		if err := repo.Create(ctx, existing); err != nil {
			t.Fatalf("Create existing profile: %v", err)
		}

		client := &mockClient{
			adults: []Member{
				{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1"},
			},
		}

		svc := NewService(repo, client)
		_, err := svc.Sync(ctx)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		p, _ := repo.GetByBSAID(ctx, "100")
		if p.Email != "local@example.com" {
			t.Errorf("expected local@example.com preserved, got %s", p.Email)
		}
	})
}

func TestSync_MarksMissingInactive(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()

	existing := &profile.Profile{
		BSAID:      "999",
		FirstName:  "Old",
		LastName:   "Profile",
		Email:      "old@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
	}

	svc := NewService(repo, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Deactivated != 1 {
		t.Errorf("expected 1 deactivated, got %d", result.Deactivated)
	}
	if result.Created != 1 {
		t.Errorf("expected 1 created, got %d", result.Created)
	}

	p, err := repo.GetByBSAID(ctx, "999")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.Status != profile.StatusInactive {
		t.Errorf("expected inactive, got %s", p.Status)
	}
}

func TestSync_Idempotent(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
	}

	svc := NewService(repo, client)

	result1, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("First Sync failed: %v", err)
	}
	if result1.Created != 1 {
		t.Errorf("expected 1 created on first sync, got %d", result1.Created)
	}

	result2, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Second Sync failed: %v", err)
	}
	if result2.Created != 0 {
		t.Errorf("expected 0 created on second sync, got %d", result2.Created)
	}
	if result2.Deactivated != 0 {
		t.Errorf("expected 0 deactivated on second sync, got %d", result2.Deactivated)
	}
}

type mockProfileRepository struct {
	mu       sync.RWMutex
	profiles map[string]*profile.Profile
}

func newMockProfileRepository() *mockProfileRepository {
	return &mockProfileRepository{
		profiles: make(map[string]*profile.Profile),
	}
}

func (r *mockProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == "" {
		p.ID = fmt.Sprintf("id-%s", p.BSAID)
	}
	clone := *p
	r.profiles[clone.ID] = &clone
	return nil
}

func (r *mockProfileRepository) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[id]
	if !ok {
		return nil, errors.New("profile not found")
	}
	clone := *p
	return &clone, nil
}

func (r *mockProfileRepository) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.BSAID == bsaID {
			clone := *p
			return &clone, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (r *mockProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.profiles[p.ID]; !ok {
		return errors.New("profile not found")
	}
	clone := *p
	r.profiles[clone.ID] = &clone
	return nil
}

func (r *mockProfileRepository) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.Email == email {
			clone := *p
			return &clone, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (r *mockProfileRepository) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}

func (r *mockProfileRepository) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*profile.Profile
	for _, p := range r.profiles {
		clone := *p
		result = append(result, &clone)
	}
	return result, nil
}

func (r *mockProfileRepository) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*profile.Profile
	for _, p := range r.profiles {
		if p.Status == status {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}
