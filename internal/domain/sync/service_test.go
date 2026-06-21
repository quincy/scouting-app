package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
)

var _ rbac.Repository = (*mockRBACRepository)(nil)

type mockRBACRepository struct {
	mu              sync.RWMutex
	roles           map[string]*rbac.Role
	permissions     map[string]*rbac.Permission
	userRoles       map[string][]string
	rolePermissions map[string][]string
}

func newMockRBACRepository() *mockRBACRepository {
	r := &mockRBACRepository{
		roles:           make(map[string]*rbac.Role),
		permissions:     make(map[string]*rbac.Permission),
		userRoles:       make(map[string][]string),
		rolePermissions: make(map[string][]string),
	}
	_ = r.SeedRoles(context.Background())
	return r
}

func (r *mockRBACRepository) SeedRoles(ctx context.Context) error {
	permIDs := make(map[string]string)
	for _, permName := range []string{"event:create", "event:view", "event:signup", "event:withdraw"} {
		perm := &rbac.Permission{Name: permName}
		_ = r.CreatePermission(ctx, perm)
		permIDs[permName] = perm.ID
	}

	roleDefs := []struct {
		Name        string
		Permissions []string
	}{
		{Name: "admin", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
		{Name: "Scoutmaster", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
		{Name: "Assistant Scoutmaster", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
		{Name: "Troop Admin", Permissions: []string{}},
		{Name: "Committee Chair", Permissions: []string{}},
		{Name: "Scouts BSA", Permissions: []string{"event:view", "event:signup", "event:withdraw"}},
		{Name: "parent", Permissions: []string{"event:view", "event:signup", "event:withdraw"}},
	}
	for _, rd := range roleDefs {
		role := &rbac.Role{Name: rd.Name}
		_ = r.CreateRole(ctx, role)
		for _, pn := range rd.Permissions {
			_ = r.LinkPermissionToRole(ctx, role.ID, permIDs[pn])
		}
	}
	return nil
}

func (r *mockRBACRepository) CreateRole(ctx context.Context, role *rbac.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rl := range r.roles {
		if rl.Name == role.Name {
			role.ID = rl.ID
			return nil
		}
	}
	if role.ID == "" {
		role.ID = fmt.Sprintf("role-%s", role.Name)
	}
	r.roles[role.ID] = role
	return nil
}

func (r *mockRBACRepository) CreatePermission(ctx context.Context, perm *rbac.Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.permissions {
		if p.Name == perm.Name {
			perm.ID = p.ID
			return nil
		}
	}
	if perm.ID == "" {
		perm.ID = fmt.Sprintf("perm-%s", perm.Name)
	}
	r.permissions[perm.ID] = perm
	return nil
}

func (r *mockRBACRepository) AssignRoleToUser(ctx context.Context, userID string, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rid := range r.userRoles[userID] {
		if rid == roleID {
			return nil
		}
	}
	r.userRoles[userID] = append(r.userRoles[userID], roleID)
	return nil
}

func (r *mockRBACRepository) RemoveRoleFromUser(ctx context.Context, userID string, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rids := r.userRoles[userID]
	for i, rid := range rids {
		if rid == roleID {
			r.userRoles[userID] = append(rids[:i], rids[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *mockRBACRepository) LinkPermissionToRole(ctx context.Context, roleID string, permID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, pid := range r.rolePermissions[roleID] {
		if pid == permID {
			return nil
		}
	}
	r.rolePermissions[roleID] = append(r.rolePermissions[roleID], permID)
	return nil
}

func (r *mockRBACRepository) GetUserRoles(ctx context.Context, userID string) ([]*rbac.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rids := r.userRoles[userID]
	var roles []*rbac.Role
	for _, rid := range rids {
		if role, ok := r.roles[rid]; ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *mockRBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]*rbac.Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rids := r.userRoles[userID]
	permSet := make(map[string]bool)
	var permissions []*rbac.Permission
	for _, rid := range rids {
		pids := r.rolePermissions[rid]
		for _, pid := range pids {
			if !permSet[pid] {
				permSet[pid] = true
				if perm, ok := r.permissions[pid]; ok {
					permissions = append(permissions, perm)
				}
			}
		}
	}
	return permissions, nil
}

func (r *mockRBACRepository) GetRoleByName(ctx context.Context, name string) (*rbac.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, role := range r.roles {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, fmt.Errorf("role %q not found", name)
}

func (r *mockRBACRepository) ListAllRoles(ctx context.Context) ([]*rbac.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var roles []*rbac.Role
	for _, role := range r.roles {
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *mockRBACRepository) GetUsersByRoleName(ctx context.Context, name string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	roleID := ""
	for _, role := range r.roles {
		if role.Name == name {
			roleID = role.ID
			break
		}
	}
	if roleID == "" {
		return nil, fmt.Errorf("role %q not found", name)
	}
	var userIDs []string
	for uid, rids := range r.userRoles {
		for _, rid := range rids {
			if rid == roleID {
				userIDs = append(userIDs, uid)
				break
			}
		}
	}
	return userIDs, nil
}

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
	rbac := newMockRBACRepository()
	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", Nickname: "Johnny", Gender: "M", PersonGUID: "guid-1", Email: "john@example.com", Phone: "555-0100", BirthDate: "1990-01-15", Positions: "Scoutmaster, Troop Admin"},
		},
		youths: nil,
	}

	svc := NewService(repo, rbac, client)
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

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	if result.Profiles[0].Status != "created" {
		t.Errorf("expected status created, got %s", result.Profiles[0].Status)
	}
	if result.Profiles[0].Old != nil {
		t.Errorf("expected Old to be nil for created profile")
	}
	if result.Profiles[0].New.FirstName != "John" {
		t.Errorf("expected snapshot FirstName John, got %s", result.Profiles[0].New.FirstName)
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
	rbac := newMockRBACRepository()
	client := &mockClient{
		adults: nil,
		youths: []Member{
			{MemberID: "200", FirstName: "Jimmy", LastName: "Jones", PersonGUID: "guid-2"},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Created != 1 {
		t.Fatalf("expected 1 created, got %d", result.Created)
	}

	if len(result.Profiles) != 1 || result.Profiles[0].Status != "created" {
		t.Errorf("expected 1 created profile in report, got %d", len(result.Profiles))
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
	rbac := newMockRBACRepository()

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
		youths: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
	}

	svc := NewService(repo, rbac, client)
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
	rbac := newMockRBACRepository()

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

	svc := NewService(repo, rbac, client)
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

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	rep := result.Profiles[0]
	if rep.Status != "updated" {
		t.Errorf("expected status updated, got %s", rep.Status)
	}
	if rep.Old == nil {
		t.Fatal("expected Old snapshot for updated profile")
	}
	if rep.Old.FirstName != "OldFirst" {
		t.Errorf("expected old first name OldFirst, got %s", rep.Old.FirstName)
	}
	if rep.New.FirstName != "John" {
		t.Errorf("expected new first name John, got %s", rep.New.FirstName)
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
	rbac := newMockRBACRepository()

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

		svc := NewService(repo, rbac, client)
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

		svc := NewService(repo, rbac, client)
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
	rbac := newMockRBACRepository()

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

	svc := NewService(repo, rbac, client)
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

	if len(result.Profiles) != 2 {
		t.Fatalf("expected 2 profiles in report, got %d", len(result.Profiles))
	}
	var deactivated *ProfileReport
	for i := range result.Profiles {
		if result.Profiles[i].Status == "deactivated" {
			deactivated = &result.Profiles[i]
		}
	}
	if deactivated == nil {
		t.Fatal("expected deactivated profile in report")
	}
	if deactivated.Old == nil {
		t.Fatal("expected Old snapshot for deactivated profile")
	}
	if deactivated.Old.Status != profile.StatusActive {
		t.Errorf("expected old status active, got %s", deactivated.Old.Status)
	}
	if deactivated.New.Status != profile.StatusInactive {
		t.Errorf("expected new status inactive, got %s", deactivated.New.Status)
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
	rbac := newMockRBACRepository()
	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com"},
		},
	}

	svc := NewService(repo, rbac, client)

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
	if len(result2.Profiles) != 0 {
		t.Errorf("expected 0 profiles in report on idempotent sync, got %d", len(result2.Profiles))
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

func TestSync_ReconcileRoles_AddsPositionRoles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	existing := &profile.Profile{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "Old Position",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com", Positions: "Scoutmaster, Committee Chair"},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.RolesAdded != 2 {
		t.Errorf("expected 2 roles added, got %d", result.RolesAdded)
	}
	if result.RolesRemoved != 0 {
		t.Errorf("expected 0 roles removed, got %d", result.RolesRemoved)
	}

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	rep := result.Profiles[0]
	if len(rep.RolesAdded) != 2 {
		t.Errorf("expected 2 roles added in report, got %d", len(rep.RolesAdded))
	}
	addedSet := make(map[string]bool)
	for _, r := range rep.RolesAdded {
		addedSet[r] = true
	}
	if !addedSet["Scoutmaster"] {
		t.Error("expected Scoutmaster in RolesAdded")
	}
	if !addedSet["Committee Chair"] {
		t.Error("expected Committee Chair in RolesAdded")
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	if !roleNames["Scoutmaster"] {
		t.Error("expected Scoutmaster role to be assigned")
	}
	if !roleNames["Committee Chair"] {
		t.Error("expected Committee Chair role to be assigned")
	}
}

func TestSync_ReconcileRoles_RemovesStaleRoles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	existing := &profile.Profile{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "Scoutmaster, Troop Admin",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	scoutmasterRole, _ := rbac.GetRoleByName(ctx, "Scoutmaster")
	adminRole, _ := rbac.GetRoleByName(ctx, "Troop Admin")
	if scoutmasterRole == nil || adminRole == nil {
		t.Fatal("could not find seeded roles")
	}
	_ = rbac.AssignRoleToUser(ctx, uid, scoutmasterRole.ID)
	_ = rbac.AssignRoleToUser(ctx, uid, adminRole.ID)

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com", Positions: "Scoutmaster"},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.RolesAdded != 0 {
		t.Errorf("expected 0 roles added, got %d", result.RolesAdded)
	}
	if result.RolesRemoved != 1 {
		t.Errorf("expected 1 role removed, got %d", result.RolesRemoved)
	}

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	rep := result.Profiles[0]
	if len(rep.RolesRemoved) != 1 || rep.RolesRemoved[0] != "Troop Admin" {
		t.Errorf("expected Troop Admin in RolesRemoved, got %v", rep.RolesRemoved)
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	if !roleNames["Scoutmaster"] {
		t.Error("expected Scoutmaster role to remain")
	}
	if roleNames["Troop Admin"] {
		t.Error("expected Troop Admin role to be removed")
	}
}

func TestSync_ReconcileRoles_DoesNotTouchProtectedRoles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	existing := &profile.Profile{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "Scoutmaster",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	scoutmasterRole, _ := rbac.GetRoleByName(ctx, "Scoutmaster")
	parentRole, _ := rbac.GetRoleByName(ctx, "parent")
	if scoutmasterRole == nil || parentRole == nil {
		t.Fatal("could not find seeded roles")
	}
	_ = rbac.AssignRoleToUser(ctx, uid, scoutmasterRole.ID)
	_ = rbac.AssignRoleToUser(ctx, uid, parentRole.ID)

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com", Positions: ""},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.RolesAdded != 0 {
		t.Errorf("expected 0 roles added, got %d", result.RolesAdded)
	}
	if result.RolesRemoved != 1 {
		t.Errorf("expected 1 role removed (Scoutmaster only), got %d", result.RolesRemoved)
	}

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	rep := result.Profiles[0]
	if len(rep.RolesRemoved) != 1 || rep.RolesRemoved[0] != "Scoutmaster" {
		t.Errorf("expected Scoutmaster in RolesRemoved, got %v", rep.RolesRemoved)
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	if roleNames["Scoutmaster"] {
		t.Error("expected Scoutmaster role to be removed")
	}
	if !roleNames["parent"] {
		t.Error("expected parent role to be preserved")
	}
}

func TestSync_ReconcileRoles_AutoCreatesUnknownPosition(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	existing := &profile.Profile{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "Old Position",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create existing profile: %v", err)
	}

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com", Positions: "New Unknown Position"},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.RolesAdded != 1 {
		t.Errorf("expected 1 role added, got %d", result.RolesAdded)
	}

	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile in report, got %d", len(result.Profiles))
	}
	rep := result.Profiles[0]
	if len(rep.RolesAdded) != 1 || rep.RolesAdded[0] != "New Unknown Position" {
		t.Errorf("expected New Unknown Position in RolesAdded, got %v", rep.RolesAdded)
	}

	role, err := rbac.GetRoleByName(ctx, "New Unknown Position")
	if err != nil {
		t.Fatalf("expected role %q to be auto-created, but GetRoleByName failed: %v", "New Unknown Position", err)
	}
	if role == nil {
		t.Fatal("expected role to exist")
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	if !roleNames["New Unknown Position"] {
		t.Error("expected New Unknown Position role to be assigned")
	}
}

func TestSync_ReconcileRoles_NoopWhenNoUserID(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	client := &mockClient{
		adults: []Member{
			{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-1", Email: "john@example.com", Positions: "Scoutmaster"},
		},
	}

	svc := NewService(repo, rbac, client)
	result, err := svc.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if result.RolesAdded != 0 {
		t.Errorf("expected 0 roles added (no userID), got %d", result.RolesAdded)
	}
	if result.RolesRemoved != 0 {
		t.Errorf("expected 0 roles removed (no userID), got %d", result.RolesRemoved)
	}
}

func TestRevert_RestoresProfileFields(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	birthdate := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	existing := &profile.Profile{
		ID:         "p1",
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		Birthdate:  birthdate,
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	svc := NewService(repo, rbac, &mockClient{})

	oldSnapshot := ProfileSnapshot{
		BSAID:      "100",
		FirstName:  "Reverted",
		LastName:   "Name",
		Nickname:   "Nick",
		Gender:     "M",
		Email:      "reverted@example.com",
		Phone:      "555-0000",
		Birthdate:  birthdate,
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		Positions:  "Scoutmaster",
	}

	if err := svc.Revert(ctx, oldSnapshot, nil, nil); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.FirstName != "Reverted" {
		t.Errorf("expected FirstName Reverted, got %s", p.FirstName)
	}
	if p.LastName != "Name" {
		t.Errorf("expected LastName Name, got %s", p.LastName)
	}
	if p.Nickname != "Nick" {
		t.Errorf("expected Nickname Nick, got %s", p.Nickname)
	}
	if p.Gender != "M" {
		t.Errorf("expected Gender M, got %s", p.Gender)
	}
	if p.Email != "reverted@example.com" {
		t.Errorf("expected Email reverted@example.com, got %s", p.Email)
	}
	if p.Phone != "555-0000" {
		t.Errorf("expected Phone 555-0000, got %s", p.Phone)
	}
	if p.Positions != "Scoutmaster" {
		t.Errorf("expected Positions Scoutmaster, got %s", p.Positions)
	}
	if p.Status != profile.StatusActive {
		t.Errorf("expected Status active, got %s", p.Status)
	}
}

func TestRevert_ReversesRoleChanges(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	existing := &profile.Profile{
		ID:         "p1",
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "Scoutmaster, Committee Chair",
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	scoutmasterRole, _ := rbac.GetRoleByName(ctx, "Scoutmaster")
	chairRole, _ := rbac.GetRoleByName(ctx, "Committee Chair")
	_ = rbac.AssignRoleToUser(ctx, uid, scoutmasterRole.ID)
	_ = rbac.AssignRoleToUser(ctx, uid, chairRole.ID)

	svc := NewService(repo, rbac, &mockClient{})

	oldSnapshot := ProfileSnapshot{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &uid,
		Positions:  "",
	}

	rolesAdded := []string{"Scoutmaster", "Committee Chair"}

	if err := svc.Revert(ctx, oldSnapshot, rolesAdded, nil); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	for _, r := range roles {
		if r.Name == "Scoutmaster" || r.Name == "Committee Chair" {
			t.Errorf("expected role %s to be removed by revert, but it still exists", r.Name)
		}
	}
}

func TestRevert_ReactivateAndRestoreRoles(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	uid := "user-1"
	birthdate := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	existing := &profile.Profile{
		ID:         "p1",
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusInactive,
		UserID:     &uid,
		Birthdate:  birthdate,
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	svc := NewService(repo, rbac, &mockClient{})

	oldSnapshot := ProfileSnapshot{
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		Birthdate:  birthdate,
		UserID:     &uid,
		Positions:  "Scoutmaster",
	}

	rolesRemoved := []string{"Scoutmaster"}

	if err := svc.Revert(ctx, oldSnapshot, nil, rolesRemoved); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.Status != profile.StatusActive {
		t.Errorf("expected profile to be reactivated, got %s", p.Status)
	}

	roles, err := rbac.GetUserRoles(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	hasScoutmaster := false
	for _, r := range roles {
		if r.Name == "Scoutmaster" {
			hasScoutmaster = true
		}
	}
	if !hasScoutmaster {
		t.Error("expected Scoutmaster role to be restored by revert")
	}
}

func TestRevert_NoUserID(t *testing.T) {
	ctx := t.Context()
	repo := newMockProfileRepository()
	rbac := newMockRBACRepository()

	existing := &profile.Profile{
		ID:         "p1",
		BSAID:      "100",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create profile: %v", err)
	}

	svc := NewService(repo, rbac, &mockClient{})

	oldSnapshot := ProfileSnapshot{
		BSAID:      "100",
		FirstName:  "Reverted",
		LastName:   "Name",
		Email:      "john@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}

	if err := svc.Revert(ctx, oldSnapshot, []string{"Scoutmaster"}, nil); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	p, err := repo.GetByBSAID(ctx, "100")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if p.FirstName != "Reverted" {
		t.Errorf("expected FirstName Reverted, got %s", p.FirstName)
	}
}
