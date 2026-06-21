package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	goSync "sync"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/sync"
	"scout-app/internal/scoutbook"
)

type mockAppConfigRepo struct{}

func (m *mockAppConfigRepo) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockAppConfigRepo) Set(ctx context.Context, key, value string) error { return nil }
func (m *mockAppConfigRepo) All(ctx context.Context) (map[string]string, error) {
	return nil, nil
}

type mockSyncClient struct{}

func (m *mockSyncClient) FetchRoster(ctx context.Context, memberType sync.MemberType) ([]sync.Member, error) {
	return nil, nil
}

type mockSyncRepo struct{}

func (m *mockSyncRepo) Create(ctx context.Context, p *profile.Profile) error { return nil }
func (m *mockSyncRepo) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepo) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepo) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepo) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepo) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	return nil, nil
}
func (m *mockSyncRepo) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	return nil, nil
}
func (m *mockSyncRepo) Update(ctx context.Context, p *profile.Profile) error { return nil }

type mockRBACRepo struct{}

func (m *mockRBACRepo) SeedRoles(ctx context.Context) error                               { return nil }
func (m *mockRBACRepo) CreateRole(ctx context.Context, r *rbac.Role) error                { return nil }
func (m *mockRBACRepo) CreatePermission(ctx context.Context, p *rbac.Permission) error    { return nil }
func (m *mockRBACRepo) AssignRoleToUser(ctx context.Context, userID, roleID string) error { return nil }
func (m *mockRBACRepo) RemoveRoleFromUser(ctx context.Context, userID, roleID string) error {
	return nil
}
func (m *mockRBACRepo) LinkPermissionToRole(ctx context.Context, roleID, permID string) error {
	return nil
}
func (m *mockRBACRepo) GetUserRoles(ctx context.Context, userID string) ([]*rbac.Role, error) {
	return nil, nil
}
func (m *mockRBACRepo) GetUserPermissions(ctx context.Context, userID string) ([]*rbac.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepo) ListAllRoles(ctx context.Context) ([]*rbac.Role, error) { return nil, nil }

func (m *mockRBACRepo) GetRoleByName(ctx context.Context, name string) (*rbac.Role, error) {
	return &rbac.Role{ID: name + "-role-id", Name: name}, nil
}
func (m *mockRBACRepo) GetUsersByRoleName(ctx context.Context, name string) ([]string, error) {
	return nil, nil
}

var mockRBAC = &mockRBACRepo{}

func TestSyncHandler_AdminPage_NoToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	req := httptest.NewRequest("GET", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.AdminPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "LOGIN_DATA") {
		t.Error("expected LOGIN_DATA form on page")
	}
}

func TestSyncHandler_AdminPage_WithToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	handler.mu.Lock()
	handler.token = &storedToken{
		token:      "test-token",
		personGUID: "guid-123",
		expiresAt:  time.Now().Add(1 * time.Hour),
	}
	handler.mu.Unlock()

	req := httptest.NewRequest("GET", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.AdminPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Token configured") {
		t.Error("expected token configured message")
	}
	if !strings.Contains(rr.Body.String(), "Start Sync") {
		t.Error("expected sync button")
	}
}

func TestSyncHandler_StoreToken_FormEncoded(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	body := `login_data=%7B%22token%22%3A%22eyJtest%22%2C%22personGuid%22%3A%22guid-abc%22%7D`
	req := httptest.NewRequest("POST", "/admin/sync/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.StoreToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Token configured") {
		t.Error("expected token configured after store")
	}
}

func TestSyncHandler_StoreToken_JSON(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	body := `{"login_data":"{\"token\":\"eyJtest\",\"personGuid\":\"guid-abc\"}"}`
	req := httptest.NewRequest("POST", "/admin/sync/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.StoreToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Token configured") {
		t.Error("expected token configured after store")
	}
}

func TestSyncHandler_StoreToken_MissingToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	body := `login_data=%7B%22personGuid%22%3A%22guid-abc%22%7D`
	req := httptest.NewRequest("POST", "/admin/sync/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.StoreToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "must contain") {
		t.Error("expected error message about missing fields")
	}
}

func TestSyncHandler_StoreToken_InvalidInnerJSON(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	body := `login_data=not-json`
	req := httptest.NewRequest("POST", "/admin/sync/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.StoreToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Could not parse") {
		t.Error("expected error message about parse failure")
	}
}

func TestSyncHandler_Sync_NoToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	req := httptest.NewRequest("POST", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.Sync(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "No Scoutbook token configured") {
		t.Error("expected error about missing token")
	}
}

func TestSyncHandler_Sync_WithToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	handler.mu.Lock()
	handler.token = &storedToken{
		token:      "test-token",
		personGUID: "guid-123",
		expiresAt:  time.Now().Add(1 * time.Hour),
	}
	handler.mu.Unlock()

	req := httptest.NewRequest("POST", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.Sync(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Sync Results") {
		t.Error("expected sync results on page")
	}
}

func TestSyncHandler_Sync_ExpiredToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, mockRBAC, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	handler.mu.Lock()
	handler.token = &storedToken{
		token:      "expired-token",
		personGUID: "guid-123",
		expiresAt:  time.Now().Add(-1 * time.Hour),
	}
	handler.mu.Unlock()

	req := httptest.NewRequest("POST", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.Sync(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (renders page with error), got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Token has expired") {
		t.Error("expected expired token message")
	}

	handler.mu.RLock()
	purged := handler.token == nil
	handler.mu.RUnlock()
	if !purged {
		t.Error("expected token to be purged after expiry")
	}
}

func TestSyncHandler_Revert(t *testing.T) {
	repo := newMockSyncRepoWithProfile()
	rbac := &mockRBACRepo{}
	svc := sync.NewService(repo, rbac, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client, &mockAppConfigRepo{})

	body := "member_id=100&name=John+Doe&old_bsa_id=100&old_first_name=RevertedJohn&old_last_name=RevertedDoe&old_member_type=adult&old_status=active"
	req := httptest.NewRequest("POST", "/admin/sync/revert", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.Revert(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "reverted") {
		t.Error("expected reverted card in response")
	}
	if !strings.Contains(rr.Body.String(), "toast") {
		t.Error("expected toast in response")
	}

	repo.mu.RLock()
	p, ok := repo.profiles["p1"]
	repo.mu.RUnlock()
	if !ok {
		t.Fatal("expected profile to exist")
	}
	if p.FirstName != "RevertedJohn" {
		t.Errorf("expected profile to be reverted to RevertedJohn, got %s", p.FirstName)
	}
}

type mockSyncRepoWithProfile struct {
	mu       goSync.RWMutex
	profiles map[string]*profile.Profile
}

func newMockSyncRepoWithProfile() *mockSyncRepoWithProfile {
	return &mockSyncRepoWithProfile{
		profiles: map[string]*profile.Profile{
			"100": {
				ID:         "p1",
				BSAID:      "100",
				FirstName:  "John",
				LastName:   "Doe",
				MemberType: profile.MemberTypeAdult,
				Status:     profile.StatusActive,
			},
		},
	}
}

func (m *mockSyncRepoWithProfile) Create(ctx context.Context, p *profile.Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *p
	if clone.ID == "" {
		clone.ID = "id-" + clone.BSAID
	}
	m.profiles[clone.ID] = &clone
	m.profiles[clone.BSAID] = &clone
	return nil
}
func (m *mockSyncRepoWithProfile) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profiles[id]
	if !ok {
		return nil, errors.New("not found")
	}
	clone := *p
	return &clone, nil
}
func (m *mockSyncRepoWithProfile) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepoWithProfile) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profiles[bsaID]
	if !ok {
		return nil, errors.New("not found")
	}
	clone := *p
	return &clone, nil
}
func (m *mockSyncRepoWithProfile) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}
func (m *mockSyncRepoWithProfile) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	return nil, nil
}
func (m *mockSyncRepoWithProfile) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	return nil, nil
}
func (m *mockSyncRepoWithProfile) Update(ctx context.Context, p *profile.Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *p
	m.profiles[clone.ID] = &clone
	m.profiles[clone.BSAID] = &clone
	return nil
}

func TestParseJWTExpiry(t *testing.T) {
	now := time.Now()
	exp := now.Add(2 * time.Hour).Unix()

	payload := map[string]any{"exp": exp}
	payloadJSON, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadJSON)
	jwt := "header." + encoded + ".signature"

	result := parseJWTExpiry(jwt)
	if result.Unix() != exp {
		t.Errorf("expected %d, got %d", exp, result.Unix())
	}
}

func TestParseJWTExpiry_InvalidJWT(t *testing.T) {
	result := parseJWTExpiry("not-a-jwt")
	if result.Before(time.Now()) {
		t.Error("expected fallback expiry in the future")
	}
}
