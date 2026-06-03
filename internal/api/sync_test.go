package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/sync"
)

type mockSyncClient struct{}

func (m *mockSyncClient) FetchRoster(ctx context.Context, memberType sync.MemberType) ([]sync.Member, error) {
	return nil, nil
}

func (m *mockSyncClient) FetchProfile(ctx context.Context, personGUID string) (*sync.PersonProfile, error) {
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
func (m *mockSyncRepo) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	return nil, nil
}
func (m *mockSyncRepo) Update(ctx context.Context, p *profile.Profile) error { return nil }

func TestSyncHandler_Sync(t *testing.T) {
	svc := sync.NewService(&mockSyncRepo{}, &mockSyncClient{})
	handler := NewSyncHandler(svc)

	req := httptest.NewRequest("POST", "/admin/sync", nil)
	rr := httptest.NewRecorder()
	handler.Sync(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var result sync.Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Created != 0 || result.Updated != 0 || result.Deactivated != 0 {
		t.Errorf("expected all zeros, got %+v", result)
	}
}
