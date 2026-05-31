package domain

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

// Hasher provides password hashing and verification.
type Hasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) error
}

// BCryptHasher implements Hasher using bcrypt.
type BCryptHasher struct{}

func (h *BCryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (h *BCryptHasher) Verify(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// MockHasher implements Hasher for testing — stores and compares plaintext.
type MockHasher struct{}

func (h *MockHasher) Hash(password string) (string, error) {
	return password, nil
}

func (h *MockHasher) Verify(password, hash string) error {
	if password != hash {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return nil
}

// Session keys
const sessionName = "session"
const sessionUserIDKey = "user_id"

// AuthService owns sessions and password verification.
type AuthService struct {
	users   UserRepository
	rbac    RBACRepository
	hasher  Hasher
	session *sessions.CookieStore
}

// NewAuthService creates an AuthService with an encrypted cookie store.
func NewAuthService(users UserRepository, rbac RBACRepository, hasher Hasher, sessionSecret string) *AuthService {
	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	return &AuthService{
		users:   users,
		rbac:    rbac,
		hasher:  hasher,
		session: store,
	}
}

// Login verifies credentials and creates a session.
func (s *AuthService) Login(w http.ResponseWriter, r *http.Request, email, password string) (*User, error) {
	user, err := s.users.GetByEmail(r.Context(), email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	if err := s.hasher.Verify(password, user.PasswordHash); err != nil {
		return nil, errors.New("invalid credentials")
	}

	sess, err := s.session.Get(r, sessionName)
	if err != nil {
		return nil, err
	}
	sess.Values[sessionUserIDKey] = user.ID
	sess.Options.MaxAge = 86400 // 24 hours
	if err := sess.Save(r, w); err != nil {
		return nil, err
	}

	return user, nil
}

// GetAuthenticatedUser reads the session and returns the logged-in user.
func (s *AuthService) GetAuthenticatedUser(r *http.Request) (*User, error) {
	sess, err := s.session.Get(r, sessionName)
	if err != nil {
		return nil, nil
	}

	userID, ok := sess.Values[sessionUserIDKey].(string)
	if !ok || userID == "" {
		return nil, nil
	}

	user, err := s.users.GetByID(r.Context(), userID)
	if err != nil {
		return nil, nil
	}
	return user, nil
}

// Logout clears the session cookie.
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.session.Get(r, sessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1 // delete cookie
	delete(sess.Values, sessionUserIDKey)
	return sess.Save(r, w)
}

// defaultRoles and defaultPermissions mirror what the SQL migration sets up.
var defaultRoles = []struct {
	Name        string
	Permissions []string
}{
	{Name: "admin", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
	{Name: "scoutmaster", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
	{Name: "asst_scoutmaster", Permissions: []string{"event:create", "event:view", "event:signup", "event:withdraw"}},
	{Name: "scout", Permissions: []string{"event:view", "event:signup", "event:withdraw"}},
	{Name: "parent", Permissions: []string{"event:view", "event:signup", "event:withdraw"}},
}

// SeedRoles creates the default roles and permissions in the RBAC repository.
func SeedRoles(ctx context.Context, rbac RBACRepository) error {
	permIDs := make(map[string]string)
	for _, permName := range []string{"event:create", "event:view", "event:signup", "event:withdraw"} {
		perm := &Permission{Name: permName}
		if err := rbac.CreatePermission(ctx, perm); err != nil {
			return err
		}
		permIDs[permName] = perm.ID
	}

	for _, rl := range defaultRoles {
		role := &Role{Name: rl.Name}
		if err := rbac.CreateRole(ctx, role); err != nil {
			return err
		}
		for _, permName := range rl.Permissions {
			if err := rbac.LinkPermissionToRole(ctx, role.ID, permIDs[permName]); err != nil {
				return err
			}
		}
	}
	return nil
}

// SeedAdminUser creates the admin user with the admin role (for mock mode).
func (s *AuthService) SeedAdminUser(ctx context.Context) error {
	hash, err := s.hasher.Hash("password")
	if err != nil {
		return err
	}
	user := &User{
		Email:        "admin@scout.local",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return err
	}

	adminRole, err := s.rbac.GetRoleByName(ctx, "admin")
	if err != nil {
		return err
	}

	return s.rbac.AssignRoleToUser(ctx, user.ID, adminRole.ID)
}
