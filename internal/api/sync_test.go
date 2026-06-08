package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/sync"
	"scout-app/internal/scoutbook"
)

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

func TestSyncHandler_AdminPage_NoToken(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	client := scoutbook.NewClient("http://example.com", "", "")
	handler := NewSyncHandler(svc, client)

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
