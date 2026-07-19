package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/user"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var _ Hasher = (*BCryptHasher)(nil)
var _ Hasher = (*MockHasher)(nil)

type Hasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) error
}

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

const SessionName = "session"
const sessionUserIDKey = "user_id"

type AuthService struct {
	users    user.Repository
	profiles profile.Repository
	rbac     rbac.Repository
	hasher   Hasher
	session  *sessions.CookieStore
}

func NewAuthService(users user.Repository, profiles profile.Repository, rbac rbac.Repository, hasher Hasher, store *sessions.CookieStore) *AuthService {
	return &AuthService{
		users:    users,
		profiles: profiles,
		rbac:     rbac,
		hasher:   hasher,
		session:  store,
	}
}

func NewCookieStore(sessionSecret string) *sessions.CookieStore {
	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	return store
}

func (s *AuthService) Login(w http.ResponseWriter, r *http.Request, email, password string) (*user.User, error) {
	u, err := s.users.GetByEmail(r.Context(), email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if u == nil {
		return nil, errors.New("invalid credentials")
	}
	if err := s.hasher.Verify(password, u.PasswordHash); err != nil {
		return nil, errors.New("invalid credentials")
	}
	if err := s.checkProfileActive(r.Context(), u.ID); err != nil {
		return nil, err
	}
	sess, err := s.session.Get(r, SessionName)
	if err != nil {
		return nil, err
	}
	sess.Values[sessionUserIDKey] = u.ID
	sess.Options.MaxAge = 86400
	if err := sess.Save(r, w); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *AuthService) GetAuthenticatedUser(r *http.Request) (*user.User, error) {
	sess, err := s.session.Get(r, SessionName)
	if err != nil {
		return nil, nil
	}
	userID, ok := sess.Values[sessionUserIDKey].(string)
	if !ok || userID == "" {
		return nil, nil
	}
	u, err := s.users.GetByID(r.Context(), userID)
	if err != nil {
		return nil, nil
	}
	return u, nil
}

func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.session.Get(r, SessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	delete(sess.Values, sessionUserIDKey)
	return sess.Save(r, w)
}

func (s *AuthService) checkProfileActive(ctx context.Context, userID string) error {
	prof, err := s.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return errors.New("invalid credentials")
	}

	if prof.Status == profile.StatusDisabled {
		return errors.New("account is disabled")
	}

	if prof == nil || prof.Status != profile.StatusInactive {
		return nil
	}
	roles, err := s.rbac.GetUserRoles(ctx, userID)
	if err != nil {
		return errors.New("invalid credentials")
	}
	for _, role := range roles {
		if role.Name == "admin" {
			return nil
		}
	}
	return errors.New("account is inactive")
}

func (s *AuthService) SeedAdminUser(ctx context.Context) error {
	hash, err := s.hasher.Hash("password")
	if err != nil {
		return err
	}
	u := &user.User{
		Email:        "admin@scout.local",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}
	if err := s.users.Create(ctx, u); err != nil {
		return err
	}
	adminRole, err := s.rbac.GetRoleByName(ctx, "admin")
	if err != nil || adminRole == nil {
		return errors.New("admin role not found")
	}
	return s.rbac.AssignRoleToUser(ctx, u.ID, adminRole.ID)
}
