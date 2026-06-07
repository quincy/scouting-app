package auth

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/user"
)

type memUserRepo struct {
	users map[string]*user.User
}

func (r *memUserRepo) Create(ctx context.Context, u *user.User) error {
	r.users[u.Email] = u
	return nil
}
func (r *memUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}
func (r *memUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	u, ok := r.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

type memRBACRepo struct{}

func (r *memRBACRepo) CreateRole(ctx context.Context, role *rbac.Role) error { return nil }
func (r *memRBACRepo) CreatePermission(ctx context.Context, perm *rbac.Permission) error {
	return nil
}
func (r *memRBACRepo) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	return nil
}
func (r *memRBACRepo) LinkPermissionToRole(ctx context.Context, roleID, permID string) error {
	return nil
}
func (r *memRBACRepo) GetUserRoles(ctx context.Context, userID string) ([]*rbac.Role, error) {
	return nil, nil
}
func (r *memRBACRepo) GetUserPermissions(ctx context.Context, userID string) ([]*rbac.Permission, error) {
	return nil, nil
}
func (r *memRBACRepo) GetRoleByName(ctx context.Context, name string) (*rbac.Role, error) {
	return &rbac.Role{ID: "admin-role", Name: "admin"}, nil
}

func TestBCryptHasher_Hash(t *testing.T) {
	hasher := &BCryptHasher{}
	hash, err := hasher.Hash("my-password")
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if hash == "my-password" {
		t.Error("hash should not equal plaintext password")
	}
}

func TestBCryptHasher_Verify_Correct(t *testing.T) {
	hasher := &BCryptHasher{}
	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}
	if err := hasher.Verify("correct-password", hash); err != nil {
		t.Errorf("Verify should succeed for correct password, got: %v", err)
	}
}

func TestBCryptHasher_Verify_Wrong(t *testing.T) {
	hasher := &BCryptHasher{}
	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}
	if err := hasher.Verify("wrong-password", hash); err == nil {
		t.Error("Verify should fail for wrong password")
	}
}

func TestMockHasher_Roundtrip(t *testing.T) {
	hasher := &MockHasher{}
	hash, err := hasher.Hash("test-password")
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}
	if hash != "test-password" {
		t.Errorf("MockHasher.Hash should return plaintext, got %q", hash)
	}
	if err := hasher.Verify("test-password", hash); err != nil {
		t.Errorf("Verify should succeed with matching password, got: %v", err)
	}
}

func TestMockHasher_Verify_Wrong(t *testing.T) {
	hasher := &MockHasher{}
	hash, _ := hasher.Hash("correct")
	if err := hasher.Verify("wrong", hash); err == nil {
		t.Error("Verify should fail for wrong password")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	u := &user.User{
		ID:           "user-1",
		Email:        "admin@scout.local",
		PasswordHash: "correct-password",
		CreatedAt:    time.Now(),
	}
	userRepo.Create(context.Background(), u)

	req := httptest.NewRequest("POST", "/login", nil)
	rr := httptest.NewRecorder()

	_, err := svc.Login(rr, req, "admin@scout.local", "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestAuthService_Login_UnknownEmail(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	req := httptest.NewRequest("POST", "/login", nil)
	rr := httptest.NewRecorder()

	_, err := svc.Login(rr, req, "unknown@scout.local", "password")
	if err == nil {
		t.Fatal("expected error for unknown email, got nil")
	}
}

func TestAuthService_GetAuthenticatedUser_NoSession(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	req := httptest.NewRequest("GET", "/events", nil)

	got, err := svc.GetAuthenticatedUser(req)
	if err != nil {
		t.Fatalf("GetAuthenticatedUser returned error: %v", err)
	}
	if got != nil {
		t.Error("expected nil user when no session, got a user")
	}
}

func TestAuthService_GetAuthenticatedUser_ValidSession(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	u := &user.User{
		ID:           "user-1",
		Email:        "admin@scout.local",
		PasswordHash: "password",
		CreatedAt:    time.Now(),
	}
	userRepo.Create(context.Background(), u)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	_, err := svc.Login(rr, req, "admin@scout.local", "password")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	cookies := rr.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/events", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}

	got, err := svc.GetAuthenticatedUser(req2)
	if err != nil {
		t.Fatalf("GetAuthenticatedUser failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Email != "admin@scout.local" {
		t.Errorf("expected email admin@scout.local, got %s", got.Email)
	}
}

func TestAuthService_Logout(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	u := &user.User{
		ID:           "user-1",
		Email:        "admin@scout.local",
		PasswordHash: "password",
		CreatedAt:    time.Now(),
	}
	userRepo.Create(context.Background(), u)

	req := httptest.NewRequest("POST", "/login", nil)
	rr := httptest.NewRecorder()
	_, err := svc.Login(rr, req, "admin@scout.local", "password")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	req2 := httptest.NewRequest("POST", "/logout", nil)
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	rr2 := httptest.NewRecorder()

	if err := svc.Logout(rr2, req2); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	req3 := httptest.NewRequest("GET", "/events", nil)
	for _, c := range rr2.Result().Cookies() {
		req3.AddCookie(c)
	}

	got, err := svc.GetAuthenticatedUser(req3)
	if err != nil {
		t.Fatalf("GetAuthenticatedUser after logout failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil user after logout")
	}
}

func TestAuthService_Login_ValidCredentials(t *testing.T) {
	userRepo := &memUserRepo{users: make(map[string]*user.User)}
	hasher := &MockHasher{}
	store := NewCookieStore("test-secret-key")
	svc := NewAuthService(userRepo, &memRBACRepo{}, hasher, store)

	u := &user.User{
		ID:           "user-1",
		Email:        "admin@scout.local",
		PasswordHash: "password",
		CreatedAt:    time.Now(),
	}
	userRepo.Create(context.Background(), u)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	got, err := svc.Login(rr, req, "admin@scout.local", "password")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Email != "admin@scout.local" {
		t.Errorf("expected email admin@scout.local, got %s", got.Email)
	}

	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}
