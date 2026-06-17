package api

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
)

func TestOnboardingHandler_WelcomePage(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard", nil)
	rr := httptest.NewRecorder()
	h.WelcomePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Welcome to Scout Events") {
		t.Error("expected welcome heading")
	}
}

func TestOnboardingHandler_PersonalPage(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard/personal", nil)
	rr := httptest.NewRecorder()
	h.PersonalPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Your Information") {
		t.Error("expected personal info heading")
	}
}

func TestOnboardingHandler_Personal_Validation(t *testing.T) {
	h := newTestOnboardingHandler()
	body := "first_name=&last_name=&email=&bsa_id="
	req := httptest.NewRequest("POST", "/onboard/personal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Personal(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with error, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "All fields are required") {
		t.Error("expected validation error")
	}
}

func TestOnboardingHandler_Personal_Success(t *testing.T) {
	h := newTestOnboardingHandler()
	body := "first_name=John&last_name=Doe&email=john@example.com&bsa_id=12345"
	req := httptest.NewRequest("POST", "/onboard/personal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Personal(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/onboard/unit" {
		t.Errorf("expected redirect to /onboard/unit, got %q", loc)
	}
}

func TestOnboardingHandler_UnitPage_Renders(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard/unit", nil)
	rr := httptest.NewRecorder()
	h.UnitPage(rr, req)

	// No session data, should redirect to onboard
	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", rr.Code)
	}
}

func TestOnboardingHandler_TimezonePage_Renders(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard/timezone", nil)
	rr := httptest.NewRecorder()
	h.TimezonePage(rr, req)

	// No session data, should redirect to onboard
	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", rr.Code)
	}
}

func TestOnboardingHandler_PasswordPage_Renders(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard/password", nil)
	rr := httptest.NewRecorder()
	h.PasswordPage(rr, req)

	// No session data, should redirect to onboard
	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", rr.Code)
	}
}

func TestOnboardingHandler_FullFlow(t *testing.T) {
	h := newTestOnboardingHandler()
	ctx := context.Background()

	// Step 1: Personal info
	body := "first_name=John&last_name=Doe&email=john@example.com&bsa_id=12345"
	req := httptest.NewRequest("POST", "/onboard/personal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Personal(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("personal step expected 302, got %d", rr.Code)
	}
	cookies := rr.Result().Cookies()

	// Step 2: Unit info
	body2 := "unit_type=Troop&unit_number=77&org_guid=abc-123"
	req2 := httptest.NewRequest("POST", "/onboard/unit", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	rr2 := httptest.NewRecorder()
	h.Unit(rr2, req2)

	if rr2.Code != http.StatusFound {
		t.Fatalf("unit step expected 302, got %d", rr2.Code)
	}
	cookies = rr2.Result().Cookies()

	// Step 3: Timezone
	body3 := "timezone=America/New_York"
	req3 := httptest.NewRequest("POST", "/onboard/timezone", strings.NewReader(body3))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req3.AddCookie(c)
	}
	rr3 := httptest.NewRecorder()
	h.Timezone(rr3, req3)

	if rr3.Code != http.StatusFound {
		t.Fatalf("timezone step expected 302, got %d", rr3.Code)
	}
	cookies = rr3.Result().Cookies()

	// Step 4: Password
	body4 := "password=testpass123&confirm_password=testpass123"
	req4 := httptest.NewRequest("POST", "/onboard/password", strings.NewReader(body4))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req4.AddCookie(c)
	}
	rr4 := httptest.NewRecorder()
	h.Password(rr4, req4)

	if rr4.Code != http.StatusFound {
		t.Fatalf("password step expected 302, got %d", rr4.Code)
	}
	loc := rr4.Header().Get("Location")
	if loc != "/onboard/complete" {
		t.Errorf("expected redirect to /onboard/complete, got %q", loc)
	}

	// Verify user was created
	users, _ := h.userRepo.(interface{ All() map[string]*user.User })
	if users != nil {
		found := false
		for _, u := range users.All() {
			if u.PasswordHash == "testpass123" {
				found = true
			}
		}
		if !found {
			t.Error("user not created")
		}
	}

	// Verify app config was saved
	orgGUID, _ := h.appConfigRepo.Get(ctx, appconfig.KeyScoutbookOrgGUID)
	if orgGUID != "abc-123" {
		t.Errorf("expected org GUID abc-123, got %q", orgGUID)
	}
	complete, _ := h.appConfigRepo.Get(ctx, appconfig.KeyOnboardingComplete)
	if complete != "true" {
		t.Errorf("expected onboarding complete, got %q", complete)
	}
}

func TestRequireOnboarding_Middleware(t *testing.T) {
	appConfigRepo := appconfig.NewInMemoryRepository()
	ctx := context.Background()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("redirects when not onboarded", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events", nil)
		rr := httptest.NewRecorder()
		RequireOnboarding(appConfigRepo, next).ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "/onboard" {
			t.Errorf("expected redirect to /onboard, got %q", rr.Header().Get("Location"))
		}
	})

	t.Run("passes through when onboarded", func(t *testing.T) {
		_ = appConfigRepo.Set(ctx, appconfig.KeyOnboardingComplete, "true")
		req := httptest.NewRequest("GET", "/events", nil)
		rr := httptest.NewRecorder()
		RequireOnboarding(appConfigRepo, next).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestRedirectIfOnboarded_Middleware(t *testing.T) {
	appConfigRepo := appconfig.NewInMemoryRepository()
	ctx := context.Background()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("passes through when not onboarded", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/onboard", nil)
		rr := httptest.NewRecorder()
		RedirectIfOnboarded(appConfigRepo, next).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("redirects when onboarded", func(t *testing.T) {
		_ = appConfigRepo.Set(ctx, appconfig.KeyOnboardingComplete, "true")
		req := httptest.NewRequest("GET", "/onboard", nil)
		rr := httptest.NewRecorder()
		RedirectIfOnboarded(appConfigRepo, next).ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "/login" {
			t.Errorf("expected redirect to /login, got %q", rr.Header().Get("Location"))
		}
	})
}

func TestOnboardingHandler_Password_Validation(t *testing.T) {
	h := newTestOnboardingHandler()

	t.Run("empty password", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/onboard/password", strings.NewReader("password=&confirm_password="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		seedOnboardingSession(h, req)
		rr := httptest.NewRecorder()
		h.Password(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 with error, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Please enter a password") {
			t.Error("expected password required error")
		}
	})

	t.Run("password too short", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/onboard/password", strings.NewReader("password=short&confirm_password=short"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		seedOnboardingSession(h, req)
		rr := httptest.NewRecorder()
		h.Password(rr, req)
		if !strings.Contains(rr.Body.String(), "at least 8 characters") {
			t.Error("expected minimum length error")
		}
	})

	t.Run("passwords do not match", func(t *testing.T) {
		body := "password=longenough1&confirm_password=different1"
		req := httptest.NewRequest("POST", "/onboard/password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		seedOnboardingSession(h, req)
		rr := httptest.NewRecorder()
		h.Password(rr, req)

		if !strings.Contains(rr.Body.String(), "Passwords do not match") {
			t.Error("expected mismatch error, got:", rr.Body.String())
		}
	})
}

func TestOnboardingHandler_CompletePage(t *testing.T) {
	h := newTestOnboardingHandler()
	req := httptest.NewRequest("GET", "/onboard/complete", nil)
	rr := httptest.NewRecorder()
	h.CompletePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "You're All Set") {
		t.Error("expected complete heading")
	}
}

func TestOnboardingHandler_Unit_Validation(t *testing.T) {
	h := newTestOnboardingHandler()
	body := "unit_type=Troop&unit_number=&org_guid="
	req := httptest.NewRequest("POST", "/onboard/unit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	seedOnboardingSession(h, req)
	rr := httptest.NewRecorder()
	h.Unit(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with error, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "All fields are required") {
		t.Error("expected validation error")
	}
}

// Helpers

func newTestOnboardingHandler() *OnboardingHandler {
	return &OnboardingHandler{
		profileRepo:   &mockProfileRepo{profiles: make(map[string]*profile.Profile)},
		userRepo:      &mockUserRepo{users: make(map[string]*user.User)},
		rbacRepo:      &mockRBACRepo{},
		appConfigRepo: appconfig.NewInMemoryRepository(),
		hasher:        &auth.MockHasher{},
		session:       auth.NewCookieStore("test-secret-key"),
		tmpl:          template.Must(template.New("").ParseFS(viewsFS, "views/onboarding_*.html")),
	}
}

func seedOnboardingSession(h *OnboardingHandler, r *http.Request) {
	w := httptest.NewRecorder()
	sess, _ := h.session.Get(r, onboardingSessionName)
	sess.Values["first_name"] = "John"
	sess.Values["last_name"] = "Doe"
	sess.Values["email"] = "john@example.com"
	sess.Values["bsa_id"] = "12345"
	sess.Values["unit_type"] = "Troop"
	sess.Values["unit_number"] = "77"
	sess.Values["org_guid"] = "abc-123"
	sess.Values["timezone"] = "America/New_York"
	_ = sess.Save(r, w)
}

type mockProfileRepo struct {
	profiles map[string]*profile.Profile
}

func (r *mockProfileRepo) Create(ctx context.Context, p *profile.Profile) error {
	if p.ID == "" {
		p.ID = "test-" + p.Email
	}
	r.profiles[p.ID] = p
	return nil
}

func (r *mockProfileRepo) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	p, ok := r.profiles[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}

func (r *mockProfileRepo) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	for _, p := range r.profiles {
		if p.Email == email {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *mockProfileRepo) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}

func (r *mockProfileRepo) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	return nil, errors.New("not found")
}

func (r *mockProfileRepo) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	var result []*profile.Profile
	for _, p := range r.profiles {
		result = append(result, p)
	}
	return result, nil
}

func (r *mockProfileRepo) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	return nil, nil
}

func (r *mockProfileRepo) Update(ctx context.Context, p *profile.Profile) error {
	r.profiles[p.ID] = p
	return nil
}

type mockUserRepo struct {
	users map[string]*user.User
}

func (r *mockUserRepo) All() map[string]*user.User {
	return r.users
}

func (r *mockUserRepo) Create(ctx context.Context, u *user.User) error {
	if u.ID == "" {
		u.ID = "test-user-" + u.PasswordHash
	}
	r.users[u.ID] = u
	return nil
}

func (r *mockUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return nil, errors.New("not found")
}
